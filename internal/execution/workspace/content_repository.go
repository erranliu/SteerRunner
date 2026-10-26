package workspace

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

// FreezeContentRepository reads objects only, never checkout files, attributes,
// filters or hooks. The destination is a fresh Run-owned directory without Git.
func FreezeContentRepository(ctx context.Context, g Git, input contracts.GenerateRepository, destination string) (*contracts.RepositorySource, error) {
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(strings.ToUpper(name), "GIT_") {
			g.Command.BlockedEnvironment = append(g.Command.BlockedEnvironment, name)
		}
	}
	g.Command.Args = append(append([]string{}, g.Command.Args...), "--no-replace-objects")
	kind, err := g.Output(ctx, input.LocalPath, "cat-file", "-t", input.Commit)
	if err != nil || strings.TrimSpace(string(kind)) != "commit" {
		return nil, errors.New("repository commit must identify an existing full commit object")
	}
	tree, err := g.Output(ctx, input.LocalPath, "rev-parse", input.Commit+"^{tree}")
	if err != nil {
		return nil, err
	}
	raw, err := g.Output(ctx, input.LocalPath, "ls-tree", "-r", "-z", "-l", "--full-tree", input.Commit)
	if err != nil {
		return nil, err
	}
	source := &contracts.RepositorySource{LocalPath: input.LocalPath, Commit: strings.ToLower(input.Commit), Tree: strings.TrimSpace(string(tree)), Files: []contracts.RepositoryFile{}}
	seen := map[string]bool{}
	components := map[string]string{}
	var batch strings.Builder
	total := 0
	for _, entry := range bytes.Split(raw, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		header, name, ok := strings.Cut(string(entry), "\t")
		fields := strings.Fields(header)
		if !ok || len(fields) != 4 || (fields[0] != "100644" && fields[0] != "100755") || fields[1] != "blob" || !contracts.ValidArtifactPath(name) || seen[strings.ToLower(name)] {
			return nil, fmt.Errorf("unsupported repository entry %q (regular safe unique paths required)", name)
		}
		// Reserve Git administration names even in nested directories.
		for _, part := range strings.Split(name, "/") {
			if strings.EqualFold(part, ".git") {
				return nil, errors.New("reserved repository path")
			}
		}
		size, err := strconv.Atoi(fields[3])
		prefix := ""
		for _, part := range strings.Split(name, "/") {
			prefix += "/" + part
			key := strings.ToLower(prefix)
			if prior, ok := components[key]; ok && prior != prefix {
				return nil, errors.New("case-colliding repository directory")
			}
			components[key] = prefix
		}
		if err != nil || size < 0 || size > contracts.MaxGenerateRepositoryBytes-total || len(source.Files) >= contracts.MaxGenerateRepositoryFiles {
			return nil, errors.New("repository_snapshot_limit_exceeded")
		}
		total += size
		seen[strings.ToLower(name)] = true
		source.Files = append(source.Files, contracts.RepositoryFile{Path: name, Blob: fields[2], SizeBytes: size})
		batch.WriteString(fields[2] + "\n")
	}
	objects, err := g.OutputInput(ctx, input.LocalPath, strings.NewReader(batch.String()), contracts.MaxGenerateRepositoryBytes+(1<<20), "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	if err := os.Mkdir(destination, 0700); err != nil {
		return nil, err
	}
	// Only the freshly created destination is removed on failed admission.
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(destination)
		}
	}()
	reader := bufio.NewReader(bytes.NewReader(objects))
	for i := range source.Files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		file := &source.Files[i]
		header, err := reader.ReadString('\n')
		if err != nil || strings.TrimSpace(header) != fmt.Sprintf("%s blob %d", file.Blob, file.SizeBytes) {
			return nil, errors.New("repository blob identity mismatch")
		}
		data := make([]byte, file.SizeBytes)
		if _, err := io.ReadFull(reader, data); err != nil {
			return nil, err
		}
		if end, err := reader.ReadByte(); err != nil || end != '\n' {
			return nil, errors.New("incomplete repository blob")
		}
		digest := sha256.Sum256(data)
		file.SHA256 = hex.EncodeToString(digest[:])
		path := filepath.Join(destination, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			return nil, err
		}
	}
	if _, err := reader.ReadByte(); err != io.EOF {
		return nil, errors.New("unexpected repository batch data")
	}
	manifest, err := json.Marshal(source)
	if err != nil || len(manifest) > contracts.MaxGenerateRepositoryManifestBytes {
		return nil, errors.New("repository_manifest_limit_exceeded")
	}
	complete = true
	return source, nil
}

// VerifyContentRepository does not consult the caller's mutable repository or
// repair a missing snapshot on recovery. Added, removed or replaced files fail.
func VerifyContentRepository(root string, source *contracts.RepositorySource) error {
	expected := map[string]contracts.RepositoryFile{}
	for _, file := range source.Files {
		expected[file.Path] = file
	}
	count := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("repository snapshot link changed")
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		file, ok := expected[filepath.ToSlash(relative)]
		if !ok || !info.Mode().IsRegular() || info.Size() != int64(file.SizeBytes) {
			return errors.New("repository snapshot file changed")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != file.SHA256 {
			return errors.New("repository snapshot digest changed")
		}
		count++
		return nil
	})
	if err != nil {
		return err
	}
	if count != len(expected) {
		return errors.New("repository snapshot file missing")
	}
	return nil
}

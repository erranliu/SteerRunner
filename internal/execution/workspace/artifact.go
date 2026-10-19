package workspace

import (
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"io"
	"io/fs"
	"os"
	"path"
)

// Root operations stay beneath an opened workspace even if path components
// are concurrently replaced. Links are rejected, including in-root links.
func artifactRoot(worktree string) (*os.Root, error) {
	root, err := os.OpenRoot(worktree)
	if err != nil {
		return nil, contracts.Fail(4, "workspace_unavailable", "Original workspace cannot be opened.")
	}
	defer root.Close()
	if err := ordinaryPath(root, contracts.ArtifactDirectory, true); err != nil {
		return nil, err
	}
	r, err := root.OpenRoot(contracts.ArtifactDirectory)
	if err != nil {
		return nil, contracts.Fail(4, "artifact_unavailable", "Artifact directory cannot be opened.")
	}
	return r, nil
}
func ordinaryPath(root *os.Root, name string, directory bool) error {
	parts := []string{}
	for current := name; current != "."; current = path.Dir(current) {
		parts = append(parts, current)
	}
	for i := len(parts) - 1; i >= 0; i-- {
		f, err := root.Lstat(parts[i])
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return contracts.Fail(5, "artifact_not_found", "Artifact file or collection directory is missing.")
			}
			return contracts.Fail(4, "artifact_unavailable", "Artifact path cannot be inspected.")
		}
		wantDir := i > 0 || directory
		if f.Mode()&os.ModeSymlink != 0 || wantDir && !f.IsDir() || !wantDir && !f.Mode().IsRegular() {
			return contracts.Fail(2, "artifact_path_invalid", "Artifact path must contain ordinary directories and a regular file; links are rejected.")
		}
	}
	return nil
}
func DiscoverArtifacts(worktree string) ([]contracts.ArtifactCandidate, error) {
	root, err := artifactRoot(worktree)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	out := []contracts.ArtifactCandidate{}
	entries := 0
	err = fs.WalkDir(root.FS(), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return contracts.Fail(4, "artifact_unavailable", "Cannot enumerate artifact directory.")
		}
		if name == "." {
			return nil
		}
		entries++
		if entries > contracts.MaxDiscoveryEntries {
			return contracts.Fail(2, "artifact_limit_exceeded", "Artifact discovery exceeds 256 entries.")
		}
		if !contracts.ValidArtifactPath(name) {
			return contracts.Fail(2, "artifact_path_invalid", "Artifact directory contains a nonportable path.")
		}
		if err := ordinaryPath(root, name, d.IsDir()); err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return contracts.Fail(4, "artifact_unavailable", "Artifact disappeared or cannot be inspected during discovery.")
			}
			if info.Size() > contracts.MaxArtifactBytes {
				return contracts.Fail(2, "artifact_limit_exceeded", "Artifact file exceeds 1 MiB.")
			}
			out = append(out, contracts.ArtifactCandidate{RelativePath: name, SizeBytes: info.Size()})
		}
		return nil
	})
	return out, err
}
func ReadArtifactSource(worktree, relative string) ([]byte, error) {
	if !contracts.ValidArtifactPath(relative) {
		return nil, contracts.Fail(2, "artifact_path_invalid", "Expected a portable relative artifact path.")
	}
	root, err := artifactRoot(worktree)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	if err := ordinaryPath(root, relative, false); err != nil {
		return nil, err
	}
	file, err := root.Open(relative)
	if err != nil {
		return nil, contracts.Fail(4, "artifact_unavailable", "Cannot open artifact file.")
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() {
		return nil, contracts.Fail(4, "artifact_unavailable", "Artifact is not a regular file.")
	}
	if before.Size() > contracts.MaxArtifactBytes {
		return nil, contracts.Fail(2, "artifact_limit_exceeded", "Artifact file exceeds 1 MiB.")
	}
	data, err := io.ReadAll(io.LimitReader(file, contracts.MaxArtifactBytes+1))
	if err != nil {
		return nil, contracts.Fail(4, "artifact_unavailable", "Cannot read complete artifact.")
	}
	if len(data) > contracts.MaxArtifactBytes {
		return nil, contracts.Fail(2, "artifact_limit_exceeded", "Artifact file exceeds 1 MiB.")
	}
	after, err := file.Stat()
	current, pathErr := root.Stat(relative)
	if err != nil || pathErr != nil || !os.SameFile(before, current) || !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) || int64(len(data)) != after.Size() {
		return nil, contracts.Fail(5, "artifact_changed", "Artifact changed during collection; publish a complete file before collecting.")
	}
	if err := ordinaryPath(root, relative, false); err != nil {
		return nil, err
	}
	return data, nil
}

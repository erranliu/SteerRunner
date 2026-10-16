package livetest

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"os"
	"path/filepath"
	"strings"
)

// ValidateRuntime verifies the complete copied public CLI runtime before and after execution.
func ValidateRuntime(m Manifest) error {
	if m.Stability == nil {
		return errors.New("missing stability runtime")
	}
	raw, err := os.ReadFile(m.Stability.RuntimeFiles.Path)
	if err != nil {
		return err
	}
	var index struct {
		Schema string `json:"schema"`
		Files  []Tool `json:"files"`
	}
	if err = contracts.DecodeStrict(raw, &index); err != nil {
		return err
	}
	files := index.Files
	if index.Schema != "steerrunner-native-runtime/v1" {
		return errors.New("invalid native runtime index schema")
	}
	if len(files) == 0 {
		return errors.New("empty native runtime index")
	}
	root := filepath.Dir(m.Stability.NativeOrca.Path)
	seen := map[string]bool{}
	for _, f := range files {
		if filepath.IsAbs(f.Path) || seen[f.Path] {
			return errors.New("invalid runtime index path")
		}
		seen[f.Path] = true
		p := filepath.Join(root, filepath.FromSlash(f.Path))
		rel, err := filepath.Rel(root, p)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return errors.New("runtime file escapes frozen root")
		}
		actual, err := filepath.EvalSymlinks(p)
		if err != nil || !strings.EqualFold(p, actual) {
			return errors.New("native runtime file missing or redirected")
		}
		body, err := os.ReadFile(p)
		sum := sha256.Sum256(body)
		if err != nil || hex.EncodeToString(sum[:]) != f.SHA256 {
			return errors.New("native runtime changed")
		}
	}
	count := 0
	err = filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	if err != nil || count != len(files) {
		return errors.New("native runtime inventory differs from frozen files")
	}
	return nil
}

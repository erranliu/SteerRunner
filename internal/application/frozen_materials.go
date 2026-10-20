package application

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

// The Record retains the original bytes and digest. Qualification never
// rebuilds material from the current checkout or edits a frozen snapshot.
func verifyFrozenMaterials(r *runstate.Record) error {
	if r.Request.Documents.Spec.Path != "" && len(r.Materials) < 2 {
		return errors.New("original frozen Spec/Plan materials are missing")
	}
	for _, material := range r.Materials {
		if material.Digest != ra.Digest([]byte(material.Content)) || !filepath.IsAbs(material.SnapshotPath) {
			return errors.New("original frozen material identity is invalid")
		}
		resolved, err := filepath.EvalSymlinks(material.SnapshotPath)
		same := filepath.Clean(resolved) == filepath.Clean(material.SnapshotPath)
		if runtime.GOOS == "windows" {
			same = strings.EqualFold(filepath.Clean(resolved), filepath.Clean(material.SnapshotPath))
		}
		if err != nil || !same {
			return errors.New("original frozen material path is unavailable or redirected")
		}
		info, err := os.Lstat(material.SnapshotPath)
		if err != nil || !info.Mode().IsRegular() || info.Size() != int64(len(material.Content)) {
			return errors.New("original frozen material file is unavailable or changed")
		}
		raw, err := os.ReadFile(material.SnapshotPath)
		if err != nil || !bytes.Equal(raw, []byte(material.Content)) {
			return errors.New("original frozen material content differs from its accepted bytes")
		}
	}
	return nil
}

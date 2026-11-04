package application

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

// Only content workflows own uploaded material snapshots. Development reads
// caller-supplied paths in its worktree and has no material qualification gate.
func verifyFrozenMaterials(r *runstate.Record) error {
	if !workflow.Content(r.Request.Workflow.ID) {
		return nil
	}
	if r.Request.Generate != nil && r.Request.Generate.Repository != nil {
		return verifyContentInputs(r)
	}
	return verifyFrozenDocuments(r)
}

// verifyContentInputs is used when the original directory is also required.
func verifyContentInputs(r *runstate.Record) error {
	if err := verifyContentDirectory(r); err != nil {
		return err
	}
	return verifyFrozenDocuments(r)
}

func verifyFrozenDocuments(r *runstate.Record) error {
	for _, material := range r.Materials {
		if material.Kind == "file" {
			if !filepath.IsAbs(material.SnapshotPath) || material.ExecutionLocation != material.SnapshotPath {
				return errors.New("original file material execution location changed")
			}
			for cursor := material.SnapshotPath; ; cursor = filepath.Dir(cursor) {
				if materialPathRedirected(cursor) {
					return errors.New("original file material path is redirected")
				}
				if cursor == filepath.Dir(cursor) {
					break
				}
			}
		}
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

// Prepare only fresh, non-redirected private input directories. Existing files
// are qualified by writeFixed and then by the original frozen-material verifier.
func checkMaterialParents(root, file string) error {
	relative, err := filepath.Rel(root, file)
	if err != nil || !filepath.IsLocal(relative) || relative == "." {
		return errors.New("file material escaped its private root")
	}
	parents := []string{}
	for cursor := filepath.Dir(file); ; cursor = filepath.Dir(cursor) {
		parents = append(parents, cursor)
		if cursor == filepath.Dir(cursor) {
			break
		}
	}
	for i := len(parents) - 1; i >= 0; i-- {
		cursor := parents[i]
		info, err := os.Lstat(cursor)
		if os.IsNotExist(err) {
			if err = os.Mkdir(cursor, 0700); err != nil {
				return err
			}
		} else if err != nil || !info.IsDir() {
			return errors.New("file material parent is not a directory")
		}
		actual, err := filepath.EvalSymlinks(cursor)
		if err != nil || materialPathRedirected(cursor) || !sameMaterialPath(actual, cursor) {
			return errors.New("file material directory is redirected")
		}
	}
	if info, err := os.Lstat(file); err == nil {
		actual, resolveErr := filepath.EvalSymlinks(file)
		if !info.Mode().IsRegular() || materialPathRedirected(file) || resolveErr != nil || !sameMaterialPath(actual, file) {
			return errors.New("file material is not a private regular file")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func sameMaterialPath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

package workspace

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func ProgressRelative(id string) string { return fileRelative(id, false) }
func ResultRelative(id string) string   { return fileRelative(id, true) }
func fileRelative(id string, result bool) string {
	if result {
		return ".steerrunner/result/" + id + "/result.bin"
	}
	return ".steerrunner/progress/" + id + "/progress.bin"
}
func ProgressDirectories(worktree, id string, prepare bool) (map[string]string, error) {
	return fileDirectories(worktree, id, prepare, false)
}
func ResultDirectories(worktree, id string, prepare bool) (map[string]string, error) {
	return fileDirectories(worktree, id, prepare, true)
}
func VerifyProgressDirectories(worktree, id string, expected map[string]string) error {
	return verifyFileDirectories(worktree, id, expected, false)
}
func InitializeProgress(worktree, id string, directories map[string]string, initial []byte) error {
	return initializeFile(worktree, id, directories, initial, false)
}
func InitializeResult(worktree, id string, directories map[string]string, initial []byte) error {
	return initializeFile(worktree, id, directories, initial, true)
}
func ReadProgress(worktree, id string, directories map[string]string) ([]byte, error) {
	return readFile(worktree, id, directories, false)
}
func ReadResult(worktree, id string, directories map[string]string) ([]byte, error) {
	return readFile(worktree, id, directories, true)
}

// Directory identities survive service restarts. A file inode may change, its
// containing directories may not. Root-relative operations prevent escapes.
func fileDirectories(worktree, id string, prepare, result bool) (map[string]string, error) {
	if !contracts.ValidRequestID(id) || filepath.Base(id) != id {
		return nil, errors.New("binding_invalid")
	}
	root, err := os.OpenRoot(worktree)
	if err != nil {
		return nil, errors.New("workspace_unavailable")
	}
	defer root.Close()
	subtree := ".steerrunner/progress"
	if result {
		subtree = ".steerrunner/result"
	}
	names := []string{".", ".steerrunner", subtree, subtree + "/" + id}
	out := map[string]string{}
	for _, name := range names {
		if prepare && name != "." {
			if err := root.Mkdir(name, 0700); err != nil && !os.IsExist(err) {
				return nil, errors.New("directory_unavailable")
			}
		}
		if err := ordinaryPath(root, name, true); err != nil {
			return nil, errors.New("binding_invalid")
		}
		identity, err := DirectoryIdentity(filepath.Join(worktree, filepath.FromSlash(name)))
		if err != nil {
			return nil, errors.New("binding_invalid")
		}
		out[name] = identity
	}
	return out, nil
}

func verifyFileDirectories(worktree, id string, expected map[string]string, result bool) error {
	current, err := fileDirectories(worktree, id, false, result)
	if err != nil {
		return err
	}
	if len(current) != len(expected) {
		return errors.New("binding_invalid")
	}
	for k, v := range current {
		if expected[k] != v {
			return errors.New("binding_invalid")
		}
	}
	return nil
}

func initializeFile(worktree, id string, directories map[string]string, initial []byte, result bool) error {
	if err := verifyFileDirectories(worktree, id, directories, result); err != nil {
		return err
	}
	root, err := os.OpenRoot(worktree)
	if err != nil {
		return errors.New("workspace_unavailable")
	}
	defer root.Close()
	name := fileRelative(id, result)
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		_, err = readFile(worktree, id, directories, result)
		return err
	}
	if err != nil {
		return errors.New("read_failed")
	}
	_, err = f.Write(initial)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return errors.New("initialization_unverified")
	}
	return verifyFileDirectories(worktree, id, directories, result)
}

func readFile(worktree, id string, directories map[string]string, result bool) ([]byte, error) {
	if err := verifyFileDirectories(worktree, id, directories, result); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(worktree)
	if err != nil {
		return nil, errors.New("workspace_unavailable")
	}
	defer root.Close()
	name := fileRelative(id, result)
	if err := ordinaryPath(root, name, false); err != nil {
		if _, e := root.Lstat(name); os.IsNotExist(e) {
			return nil, errors.New("file_missing")
		}
		return nil, errors.New("binding_invalid")
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, errors.New("read_failed")
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil || !before.Mode().IsRegular() {
		return nil, errors.New("read_failed")
	}
	if before.Size() > contracts.MaxProgressBytes {
		return nil, errors.New("size_exceeded")
	}
	data, err := io.ReadAll(io.LimitReader(f, contracts.MaxProgressBytes+1))
	if err != nil {
		return nil, errors.New("read_failed")
	}
	if len(data) > contracts.MaxProgressBytes {
		return nil, errors.New("size_exceeded")
	}
	after, err := f.Stat()
	current, pathErr := root.Lstat(name)
	if err != nil || pathErr != nil || !os.SameFile(before, current) || !os.SameFile(before, after) || current.Mode()&os.ModeSymlink != 0 || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) || int64(len(data)) != after.Size() {
		return nil, errors.New("file_changing")
	}
	// Re-read the same open regular file to detect in-place same-size/mtime edits.
	if _, err = f.Seek(0, 0); err != nil {
		return nil, errors.New("read_failed")
	}
	again, err := io.ReadAll(io.LimitReader(f, contracts.MaxProgressBytes+1))
	if err != nil || !bytes.Equal(data, again) {
		return nil, errors.New("file_changing")
	}
	if err := verifyFileDirectories(worktree, id, directories, result); err != nil {
		return nil, err
	}
	return data, nil
}

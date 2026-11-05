package metadataprocess

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNativeCatalogExecutableExcludesProjectCWD(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err = os.WriteFile(filepath.Join(dir, "catalog.exe"), body, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "."+string(os.PathListSeparator)+dir)
	for i := 0; i < 2; i++ {
		project := t.TempDir()
		if err = os.WriteFile(filepath.Join(project, "catalog.exe"), []byte("foreign"), 0700); err != nil {
			t.Fatal(err)
		}
		t.Chdir(project)
		got, err := Executable("catalog.exe")
		if err != nil || got != filepath.Join(dir, "catalog.exe") {
			t.Fatal(got, err)
		}
		if _, err = Executable("./catalog.exe"); err == nil {
			t.Fatal("relative project executable accepted")
		}
	}
}

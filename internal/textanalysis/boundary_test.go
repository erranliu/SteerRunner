package textanalysis

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestAnalysisDependencyBoundary(t *testing.T) {
	for _, root := range []string{".", "../execution", "../codexcommand"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range file.Imports {
				name, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					return err
				}
				if root == "../execution" && strings.Contains(name, "/internal/textanalysis") {
					t.Errorf("%s: execution imports text analysis", path)
				}
				if root == "." {
					for _, banned := range []string{"/application", "/runstate", "/storage", "/execution/backend", "/execution/adapters", "/execution/reliable", "/execution/workflow"} {
						if strings.Contains(name, banned) {
							t.Errorf("%s: analysis imports %s", path, name)
						}
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

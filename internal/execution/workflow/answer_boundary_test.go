package workflow

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Assembly may choose concrete implementations; application coordination may
// not decode a private question payload or select its delivery representation.
func privateAnswerKnowledge(file *ast.File) bool {
	bad := false
	ast.Inspect(file, func(node ast.Node) bool {
		if sel, ok := node.(*ast.SelectorExpr); ok {
			switch sel.Sel.Name {
			case "DecodeQuestionRoute", "AskRouteData", "QuestionRouteImplementationKey":
				bad = true
			}
		}
		if fn, ok := node.(*ast.FuncDecl); ok && fn.Name.Name == "answer" {
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "Delivery" {
					bad = true
				}
				return true
			})
		}
		return true
	})
	return bad
}
func TestSRNativeAnswerKnowledgeBoundary(t *testing.T) {
	err := filepath.WalkDir(filepath.Join("..", "..", "application"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		if privateAnswerKnowledge(file) {
			t.Errorf("application coordinates private native answer encoding: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		body string
		bad  bool
	}{
		{`func answer(){ owner.PrepareLifecycleAnswer(route,session,answer,text) }`, false},
		{`func port(){ codex.NewExecutionLifecycle(setup) }`, false},
		{`func answer(){ codex.DecodeQuestionRoute(route,session) }`, true},
		{`func helper(){ var route codex.AskRouteData }`, true},
		{`func answer(){ if route.Delivery=="async" {} }`, true},
	} {
		file, err := parser.ParseFile(token.NewFileSet(), "guard.go", "package fixture\n"+tc.body, 0)
		if err != nil {
			t.Fatal(err)
		}
		if privateAnswerKnowledge(file) != tc.bad {
			t.Fatal("private answer guard rejected assembly or missed native knowledge", tc.body)
		}
	}
}

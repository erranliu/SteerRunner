package cli

import "testing"

func TestR2CLIShape(t *testing.T) {
	for _, args := range [][]string{
		{"artifacts", "discover", "--run-id", "R"}, {"artifacts", "list", "--run-id", "R"}, {"artifacts", "read", "--run-id", "R", "--artifact-id", "A"}, {"artifacts", "collect", "--run-id", "R", "--input", "-"}, {"workspace-git", "--run-id", "R"},
	} {
		if _, f := parse(args); f != nil {
			t.Fatal(args, f)
		}
	}
	for _, args := range [][]string{
		{"artifacts", "read", "--run-id", "R"}, {"artifacts", "list", "--run-id", "R", "--artifact-id", "A"}, {"artifacts", "collect", "--run-id", "R"}, {"workspace-git", "--run-id", "R", "--input", "-"}, {"workspace-git", "--run-id", "R", "--request-id", "X"}, {"artifacts", "discover", "--run-id", "R", "--after", "X"},
	} {
		if _, f := parse(args); f == nil {
			t.Fatal("accepted unsupported shape", args)
		}
	}
}

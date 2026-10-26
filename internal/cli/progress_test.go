package cli

import "testing"

func TestProgressCLIShape(t *testing.T) {
	if _, err := parse([]string{"progress-file", "--run-id", "R"}); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"progress-file"}, {"progress-file", "--run-id", "R", "--input", "-"}, {"progress-file", "--run-id", "R", "--after", "x"}, {"progress-file", "--run-id", "R", "--artifact-id", "A"}} {
		if _, err := parse(args); err == nil {
			t.Fatal(args)
		}
	}
}

func TestResultFileCLIShapeAndFence(t *testing.T) {
	for _, args := range [][]string{{"result-file", "--run-id", "R", "--json"}, {"result-file", "--run-id", "R", "--expected-instance-id", "I"}} {
		if _, err := parse(args); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"result-file"}, {"result-file", "--run-id", "R", "--input", "-"}, {"result-file", "--request-id", "R"}, {"result-file", "--run-id", "R", "--artifact-id", "A"}, {"result-file", "--run-id", "R", "--after", "x"}} {
		if _, err := parse(args); err == nil {
			t.Fatal(args)
		}
	}
}

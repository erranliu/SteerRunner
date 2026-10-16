package cli

import "testing"

func TestEnvironmentCheckArguments(t *testing.T) {
	for _, args := range [][]string{{"environment-check", "start", "--input", "-"}, {"environment-check", "stop", "--input", "-"}, {"environment-check", "inspect", "--check-id", "original"}, {"environment-check", "inspect", "--request-id", "original"}} {
		if _, err := parse(args); err != nil {
			t.Fatal(args, err)
		}
	}
	for _, args := range [][]string{{"environment-check", "inspect"}, {"environment-check", "inspect", "--check-id", "a", "--request-id", "b"}, {"environment-check", "inspect", "--run-id", "a"}, {"environment-check", "stop", "--check-id", "a", "--input", "-"}, {"environment-check", "start"}, {"environment-check", "inspect", "--check-id", "a", "--refresh-pr"}, {"inspect", "--check-id", "a"}, {"environment-check", "inspect", "--request-id", "a", "extra"}} {
		if _, err := parse(args); err == nil {
			t.Fatal("invalid check arguments accepted", args)
		}
	}
}

package livetest

import (
	"strings"
	"testing"
)

func TestCommandAuditRejectsPermissionFailureWithSuccessfulCommit(t *testing.T) {
	good := `{"method":"item/completed","params":{"item":{"type":"commandExecution","exitCode":0,"aggregatedOutput":"[branch d3d37b4] Add evidence\n"}}}`
	for _, c := range []struct {
		name, body string
		denied     bool
	}{
		{"successful-commit", good, false},
		{"zero-exit-with-denial", strings.Replace(good, "[branch", "error: Unable to create packed-refs.lock: Permission denied\\n[branch", 1), true},
		{"windows-denial", strings.Replace(good, "[branch", "Access is denied. [branch", 1), true},
		{"nonzero-exit-with-denial", strings.Replace(strings.Replace(good, `"exitCode":0`, `"exitCode":1`, 1), "[branch", "Permission denied [branch", 1), true},
		{"commentary-is-not-command", strings.Replace(strings.Replace(good, "commandExecution", "agentMessage", 1), "[branch", "Permission denied [branch", 1), false},
	} {
		t.Run(c.name, func(t *testing.T) {
			audit, err := AuditCommandJournal(strings.NewReader(c.body))
			if (err != nil) != c.denied || (audit.PermissionErrors > 0) != c.denied {
				t.Fatalf("audit=%+v err=%v", audit, err)
			}
		})
	}
}

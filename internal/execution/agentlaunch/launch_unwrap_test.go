package agentlaunch

import (
	"strings"
	"testing"
)

func TestExactLaunchWrappersRetainHistoricalCommands(t *testing.T) {
	inner := `& 'codex.exe' 'READY'`
	for _, current := range []bool{true, false} {
		launch := teaPathLaunchVersion(inner, current)
		got, ok := UnwrapTeaPathLaunch(launch)
		if !ok || got != inner {
			t.Fatal("known launch lost", current, got, ok)
		}
		for _, bad := range []string{launch + "; other", "other; " + launch, strings.Replace(launch, "$srTeaOriginalPath=$env:PATH", "$srTeaOriginalPath='other'", 1)} {
			if _, ok := UnwrapTeaPathLaunch(bad); ok {
				t.Fatal("modified template recognized")
			}
		}
	}
	for _, run := range []string{"", strings.Repeat("a", 32)} {
		launch := ProgressCommand(inner, run)
		if got, ok := UnwrapProgressCommand(launch, run); !ok || got != inner {
			t.Fatal("known progress launch lost", got, ok)
		}
		if _, ok := UnwrapProgressCommand(launch+"; other", run); ok {
			t.Fatal("arbitrary suffix accepted")
		}
		if run != "" {
			if _, ok := UnwrapProgressCommand(launch, strings.Repeat("b", 32)); ok {
				t.Fatal("another run's progress path accepted")
			}
		}
	}
}

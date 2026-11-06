package livetest

import "testing"

func TestStructuredCLIAuditRequiresOriginalCalls(t *testing.T) {
	start := `{"kind":"intent","call_id":"original","args":["worktree","show","--worktree","id:original"]}` + "\n"
	end := `{"kind":"receipt","call_id":"original","args":["worktree","show","--worktree","id:original"],"failed":false}` + "\n"
	phase := `{"phase":"dispatch","argv":["worktree","show"]}` + "\n" + `{"phase":"result","argv":["worktree","show"],"exit":0}` + "\n"
	for _, test := range []struct {
		name, raw      string
		settled, valid bool
	}{
		{"original", start + end, true, true}, {"native-phase", phase, true, true},
		{"failed-original", start + `{"kind":"receipt","call_id":"original","args":["worktree","show","--worktree","id:original"],"failed":true}` + "\n", true, true},
		{"active-pending-read", start + end + `{"kind":"intent","call_id":"pending","args":["worktree","show"]}` + "\n", false, true},
		{"settled-pending-read", start + end + `{"kind":"intent","call_id":"pending","args":["worktree","show"]}` + "\n", true, false},
		{"empty", "", true, false}, {"unrecognized", "{}\n", true, false}, {"truncated", (start + end)[:len(start+end)-1], true, false},
		{"no-completion", start, false, false}, {"foreign-receipt", end, true, false},
		{"foreign-args", start + `{"kind":"receipt","call_id":"original","args":["worktree","show","--worktree","id:foreign"],"failed":false}` + "\n", true, false},
		{"missing-operation", start + end + `{"kind":"intent","call_id":"bad"}` + "\n", true, false},
		{"legacy-send", start + end + `{"phase":"dispatch","argv":["terminal","send"]}` + "\n", false, false},
		{"legacy-helper", start + end + `{"phase":"dispatch","argv":["terminal","show","--native-control"]}` + "\n", false, false},
		{"unknown-row", start + end + `{"kind":"other","args":["worktree","show"]}` + "\n", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateStructuredCLI([]byte(test.raw), test.settled); (err == nil) != test.valid {
				t.Fatal(test.name, err)
			}
		})
	}
}

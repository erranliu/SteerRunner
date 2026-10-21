package application

import (
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
)

func TestRunNamePrefixFreeze(t *testing.T) {
	f := newBaseRefreshFixture(t, "origin", "main")
	for _, prefix := range []string{"", "sl-103-c2", "acme-feature-7"} {
		f.request.NamePrefix = prefix
		f.request.Correlation = map[string]string{"steerline_task_id": "999", "caller": "unrelated"}
		f.input.RequestID = contracts.NewID()
		f.input.Input = canonical(f.request)
		r := f.freeze()
		want := prefix
		if want == "" {
			want = "sr"
		}
		if r.Snapshot.Session.NamePrefix != want || r.Request.NamePrefix != prefix || r.InputDigest != reliable.Digest(canonical(r.Request)) {
			t.Fatal("naming hint was not frozen independently of correlation", r.Snapshot.Session, r.Request)
		}
	}
}

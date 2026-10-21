package storage

import (
	"context"
	"testing"

	"github.com/erranliu/SteerRunner/internal/runstate"
)

func TestSRObservationSnapshotPaging(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := &runstate.Record{ID: "original", Revision: 1}
	if err := s.Commit(r, 0, nil, "accepted"); err != nil {
		t.Fatal(err)
	}
	_, first, err := s.Observe(context.Background(), r.ID, "", 1)
	if err != nil || len(first.Events) != 1 || first.HasMore {
		t.Fatal(first, err)
	}
	_, empty, err := s.Observe(context.Background(), r.ID, first.NextCursor, 1)
	if err != nil || len(empty.Events) != 0 || empty.NextCursor != first.NextCursor || empty.HasMore {
		t.Fatal("empty cursor skipped future events", empty, err)
	}
	for revision := uint64(2); revision <= 5; revision++ {
		r.Revision = revision
		if err := s.Commit(r, revision-1, nil, "changed"); err != nil {
			t.Fatal(err)
		}
	}
	cursor := empty.NextCursor
	for revision := uint64(2); revision <= 5; revision++ {
		observed, page, err := s.Observe(context.Background(), r.ID, cursor, 1)
		if err != nil || observed.Revision != 5 || len(page.Events) != 1 || page.Events[0].Revision != revision || page.NextCursor == cursor || page.HasMore != (revision < 5) {
			t.Fatal("page lost snapshot or skipped event", observed, page, err)
		}
		cursor = page.NextCursor
	}
	if _, _, err := s.Observe(context.Background(), "foreign", cursor, 1); err == nil {
		t.Fatal("foreign run accepted")
	}
	if _, _, err := s.Observe(context.Background(), r.ID, "foreign", 1); err == nil {
		t.Fatal("foreign cursor accepted")
	}
	if _, err := s.db.Exec(`UPDATE runs SET revision=999 WHERE id='original'`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Observe(context.Background(), r.ID, cursor, 1); err == nil {
		t.Fatal("inconsistent durable row accepted")
	}
}

func TestSRObservationConcurrentCommits(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := &runstate.Record{ID: "original", Revision: 1}
	if err := s.Commit(r, 0, nil, "accepted"); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		for revision := uint64(2); revision <= 25; revision++ {
			next := &runstate.Record{ID: r.ID, Revision: revision}
			if err := s.Commit(next, revision-1, nil, "changed"); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	cursor := ""
	seen := map[string]bool{}
	for calls := 0; len(seen) < 25 && calls < 1000; calls++ {
		observed, page, err := s.Observe(context.Background(), r.ID, cursor, 3)
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range page.Events {
			if event.Revision > observed.Revision || seen[event.EventID] {
				t.Fatal("out of snapshot or duplicate event", event, observed.Revision)
			}
			seen[event.EventID] = true
		}
		cursor = page.NextCursor
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if len(seen) != 25 {
		t.Fatal("cursor skipped events", len(seen))
	}
}

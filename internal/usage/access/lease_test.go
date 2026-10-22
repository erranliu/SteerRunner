package access

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
)

type journal map[string][]byte

func (j journal) UsageJournal(_ context.Context, id string, target any) error {
	v, ok := j[id]
	if !ok {
		return sql.ErrNoRows
	}
	return json.Unmarshal(v, target)
}
func (j journal) PutUsageJournal(ctx context.Context, id string, v any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	raw, e := json.Marshal(v)
	j[id] = raw
	return e
}

type port struct {
	groups                   []native.UsageGroup
	folders                  []native.UsageFolder
	unknown, offline, reused bool
	creates, deletes         int
	j                        journal
	deleteUnknown            bool
	hasProjects              bool
	deletedFolderHook        func()
	createdGroupHook         func()
	createdFolderHook        func()
}

func (p *port) Groups(context.Context) ([]native.UsageGroup, error) {
	if p.offline {
		return nil, errors.New("offline")
	}
	return p.groups, nil
}
func (p *port) Folders(context.Context) ([]native.UsageFolder, error) {
	if p.offline {
		return nil, errors.New("offline")
	}
	return p.folders, nil
}
func (p *port) CreateGroup(ctx context.Context, name string) (native.UsageGroup, error) {
	p.creates++
	if len(p.j) == 0 {
		panic("missing durable intent")
	}
	g := native.UsageGroup{ID: "g", Name: name}
	p.groups = append(p.groups, g)
	if p.createdGroupHook != nil {
		p.createdGroupHook()
	}
	if p.unknown {
		return g, errors.New("lost response")
	}
	return g, nil
}
func (p *port) CreateFolder(ctx context.Context, g, name, root string) (native.UsageFolder, error) {
	p.creates++
	f := native.UsageFolder{ID: "f", Name: name, GroupID: g, Path: root}
	p.folders = append(p.folders, f)
	if p.createdFolderHook != nil {
		p.createdFolderHook()
	}
	return f, nil
}
func (p *port) DeleteGroup(context.Context, string) error { p.deletes++; p.groups = nil; return nil }
func (p *port) DeleteFolder(context.Context, string) error {
	p.deletes++
	p.folders = nil
	if p.deletedFolderHook != nil {
		p.deletedFolderHook()
	}
	if p.deleteUnknown {
		return errors.New("delete response lost")
	}
	return nil
}
func (p *port) GroupUnused(context.Context, string) (bool, error) { return !p.hasProjects, nil }
func (p *port) SelectorRevoked(context.Context, string, string) bool {
	return !p.offline && len(p.folders) == 0
}

func TestUsageKnownOutcomeSurvivesCancellation(t *testing.T) {
	for _, stage := range []string{"group", "folder", "delete"} {
		t.Run(stage, func(t *testing.T) {
			j := journal{}
			p := &port{j: j}
			m := Manager{Store: j, Instance: "i", PolicyDigest: "p"}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if stage == "group" {
				p.createdGroupHook = cancel
			}
			if stage == "folder" {
				p.createdFolderHook = cancel
			}
			l, err := m.Acquire(ctx, p, Lease{RunID: "r", HostID: "h", Root: "root", File: "log"}, "c")
			if stage == "delete" {
				if err != nil {
					t.Fatal(err)
				}
				p.deletedFolderHook = cancel
				l, err = m.Release(ctx, p, l, "", func(context.Context, Lease) bool { return true })
			}
			if !errors.Is(err, context.Canceled) {
				t.Fatal("cancellation lost", l, err)
			}
			var durable Lease
			if err = j.UsageJournal(context.Background(), l.ID, &durable); err != nil || durable.CreateUnresolved || durable.MutationUnresolved || durable.GroupID == "" {
				t.Fatal(durable, err)
			}
			if stage == "group" && (durable.FolderID != "" || p.creates != 1) {
				t.Fatal("cancelled operation created folder", durable)
			}
			l, err = m.Release(context.Background(), p, l, "", func(context.Context, Lease) bool { return true })
			if err != nil || l.Phase != "revoked" {
				t.Fatal(l, err)
			}
			wantDeletes := 2
			if stage == "group" {
				wantDeletes = 1
			}
			if p.deletes != wantDeletes {
				t.Fatal("mutation replayed", p.deletes)
			}
		})
	}
}

func TestUsageRevokedPermitStillAllowsCleanup(t *testing.T) {
	j := journal{}
	p := &port{j: j}
	allowed := true
	m := Manager{Store: j, Instance: "i", PolicyDigest: "p", Permit: func(context.Context) error {
		if !allowed {
			return errors.New("revoked")
		}
		return nil
	}}
	l, err := m.Acquire(context.Background(), p, Lease{HostID: "h", Root: "root"}, "c")
	if err != nil {
		t.Fatal(err)
	}
	allowed = false
	if m.ValidateRead(context.Background(), p, l) == nil {
		t.Fatal("revoked permit allowed read")
	}
	if _, err = m.Acquire(context.Background(), p, l, "another"); err == nil || p.creates != 2 {
		t.Fatal("revoked permit allowed acquire")
	}
	l, err = m.Release(context.Background(), p, l, "", func(context.Context, Lease) bool { return true })
	if err != nil || l.Phase != "revoked" {
		t.Fatal("revocation blocked cleanup", l, err)
	}
}

func TestUsageRevocationBetweenCreatesStopsAuthorization(t *testing.T) {
	j := journal{}
	p := &port{j: j}
	allowed := true
	p.createdGroupHook = func() { allowed = false }
	m := Manager{Store: j, Instance: "i", PolicyDigest: "p", Permit: func(context.Context) error {
		if !allowed {
			return errors.New("revoked")
		}
		return nil
	}}
	l, err := m.Acquire(context.Background(), p, Lease{RunID: "r", HostID: "h", Root: "root"}, "c")
	if err == nil || p.creates != 1 || l.CreateUnresolved || l.FolderID != "" {
		t.Fatal("revoked permit created folder", l, err)
	}
	l, err = m.Release(context.Background(), p, l, "", func(context.Context, Lease) bool { return true })
	if err != nil || l.Phase != "revoked" || p.deletes != 1 {
		t.Fatal(l, err)
	}
}
func TestUsageLeaseReferencesAndEpoch(t *testing.T) {
	ctx := context.Background()
	j := journal{}
	p := &port{j: j}
	m := Manager{Store: j, Instance: "i", PolicyDigest: "policy"}
	l, err := m.Acquire(ctx, p, Lease{HostID: "h", Root: "root", File: "log"}, "one")
	if err != nil {
		t.Fatal(err)
	}
	l, err = m.Acquire(ctx, p, l, "two")
	if err != nil || p.creates != 2 {
		t.Fatal(l, err)
	}
	l, err = m.Release(ctx, p, l, "one", nil)
	if err != nil || p.deletes != 0 || l.Phase != "active" {
		t.Fatal(l, err)
	}
	l, err = m.Release(ctx, p, l, "two", func(context.Context, Lease) bool { return true })
	if err != nil || l.Phase != "revoked" || p.deletes != 2 {
		t.Fatal(l, err)
	}
	old := l
	p.createdGroupHook = func() {
		var intent Lease
		if err := j.UsageJournal(ctx, old.ID, &intent); err != nil || intent.Epoch != old.Epoch+1 || intent.GroupID != "" || intent.FolderID != "" {
			t.Fatal("new creation intent reused revoked resource IDs", intent, err)
		}
	}
	fresh, err := m.Acquire(ctx, p, l, "new")
	if err != nil || fresh.Epoch <= old.Epoch || fresh.Marker == old.Marker {
		t.Fatal(fresh, err)
	}
	_, err = m.Release(ctx, p, old, "", func(context.Context, Lease) bool { return true })
	if err == nil || p.deletes != 2 {
		t.Fatal("old epoch deleted fresh registration")
	}
}
func TestUsageCleanupUnknownAndOffline(t *testing.T) {
	ctx := context.Background()
	j := journal{}
	p := &port{j: j, unknown: true}
	m := Manager{Store: j, Instance: "i", PolicyDigest: "p"}
	l, err := m.Acquire(ctx, p, Lease{HostID: "h", Root: "root"}, "r")
	if err == nil {
		t.Fatal("lost create treated as absent")
	}
	p.groups = nil
	_, err = m.Acquire(ctx, p, l, "r")
	if err == nil || p.creates != 1 {
		t.Fatal("blind create replay")
	}
	_, err = m.Release(ctx, p, l, "", func(context.Context, Lease) bool { return true })
	if err == nil || p.deletes != 0 {
		t.Fatal("empty list falsely revoked")
	}
	j = journal{}
	p = &port{j: j}
	m.Store = j
	l, err = m.Acquire(ctx, p, Lease{HostID: "h", Root: "root"}, "r")
	if err != nil {
		t.Fatal(err)
	}
	p.offline = true
	l, err = m.Release(ctx, p, l, "r", func(context.Context, Lease) bool { return true })
	if err == nil || l.Phase == "revoked" {
		t.Fatal("offline falsely revoked")
	}
	p.offline = false
	p.folders[0].Name = "external reuse"
	_, err = m.Release(ctx, p, l, "", func(context.Context, Lease) bool { return true })
	if err == nil || p.deletes != 0 {
		t.Fatal("external registration deleted")
	}
}

func TestUsageCleanupLostDeleteDoesNotReplay(t *testing.T) {
	ctx := context.Background()
	j := journal{}
	p := &port{j: j, deleteUnknown: true}
	m := Manager{Store: j, Instance: "i", PolicyDigest: "p"}
	l, err := m.Acquire(ctx, p, Lease{HostID: "h", Root: "root", File: "log"}, "r")
	if err != nil {
		t.Fatal(err)
	}
	l, err = m.Release(ctx, p, l, "r", func(context.Context, Lease) bool { return true })
	if err == nil || !l.MutationUnresolved {
		t.Fatal(l, err)
	}
	l, err = m.Release(ctx, p, l, "", func(context.Context, Lease) bool { return true })
	if err == nil || l.Phase == "revoked" || p.deletes != 1 {
		t.Fatal("unknown delete replayed or falsely revoked", l, err)
	}
}

func TestUsageCleanupExternalAttachments(t *testing.T) {
	for _, field := range []string{"items", "comments"} {
		t.Run(field, func(t *testing.T) {
			ctx := context.Background()
			j := journal{}
			p := &port{j: j}
			m := Manager{Store: j, Instance: "i", PolicyDigest: "p"}
			l, err := m.Acquire(ctx, p, Lease{HostID: "h", Root: "root"}, "consumer")
			if err != nil {
				t.Fatal(err)
			}
			if field == "items" {
				p.folders[0].LinkedItems = json.RawMessage(`[{"type":"external"}]`)
			} else {
				p.folders[0].DiffComments = json.RawMessage(`[{"id":"external"}]`)
			}
			if m.ValidateRead(ctx, p, l) == nil {
				t.Fatal("external reference allowed reading")
			}
			if _, err = m.Release(ctx, p, l, "consumer", func(context.Context, Lease) bool { return true }); err == nil || p.deletes != 0 {
				t.Fatal("externally reused folder deleted", err)
			}
		})
	}
}

func TestUsageGroupCascadeReferences(t *testing.T) {
	for _, kind := range []string{"child", "repository", "late-repository", "late-rename"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			j := journal{}
			p := &port{j: j}
			m := Manager{Store: j, Instance: "i", PolicyDigest: "p"}
			l, err := m.Acquire(ctx, p, Lease{HostID: "h", Root: "root"}, "consumer")
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "child":
				parent := l.GroupID
				p.groups = append(p.groups, native.UsageGroup{ID: "external-child", ParentGroupID: &parent})
			case "repository":
				p.hasProjects = true
			case "late-repository":
				p.deletedFolderHook = func() { p.hasProjects = true }
			case "late-rename":
				p.deletedFolderHook = func() { p.groups[0].Name = "external name" }
			}
			_, err = m.Release(ctx, p, l, "consumer", func(context.Context, Lease) bool { return true })
			want := 0
			if kind == "late-repository" || kind == "late-rename" {
				want = 1
			}
			if err == nil || p.deletes != want || len(p.groups) == 0 {
				t.Fatal("external group reference cascaded", err, p.deletes)
			}
		})
	}
}

func TestUsageAuthorizationFenceRead(t *testing.T) {
	ctx := context.Background()
	j := journal{}
	p := &port{j: j}
	m := Manager{Store: j, Instance: "i", PolicyDigest: "policy"}
	l, err := m.Acquire(ctx, p, Lease{HostID: "h", Root: "root"}, "r")
	if err != nil {
		t.Fatal(err)
	}
	if err = m.ValidateRead(ctx, p, l); err != nil {
		t.Fatal(err)
	}
	replacement := "other-host"
	p.folders[0].ConnectionID = &replacement
	if err = m.ValidateRead(ctx, p, l); err == nil {
		t.Fatal("redirected folder remained readable")
	}
	p.folders[0].ConnectionID = nil
	m.PolicyDigest = "revoked"
	if err = m.ValidateRead(ctx, p, l); err == nil {
		t.Fatal("revoked policy remained readable")
	}
}

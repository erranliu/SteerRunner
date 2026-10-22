// Package access journals temporary Orca registrations. Unknown outcomes remain
// pending; a missing file or disconnected host never proves revocation.
package access

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	"github.com/erranliu/SteerRunner/internal/usage"
)

type Journal interface {
	UsageJournal(context.Context, string, any) error
	PutUsageJournal(context.Context, string, any) error
}
type Port interface {
	Groups(context.Context) ([]native.UsageGroup, error)
	Folders(context.Context) ([]native.UsageFolder, error)
	CreateGroup(context.Context, string) (native.UsageGroup, error)
	CreateFolder(context.Context, string, string, string) (native.UsageFolder, error)
	DeleteFolder(context.Context, string) error
	DeleteGroup(context.Context, string) error
	SelectorRevoked(context.Context, string, string) bool
	GroupUnused(context.Context, string) (bool, error)
}
type Lease struct {
	RunID                                                                        string
	Executable                                                                   string
	Args                                                                         []string
	ID, HostID, Environment, Runtime, Root, File, Marker, Instance, PolicyDigest string
	Epoch                                                                        uint64
	GroupID, FolderID, Phase                                                     string
	Consumers                                                                    map[string]bool
	Runs                                                                         map[string]bool
	CreateUnresolved                                                             bool
	MutationUnresolved                                                           bool
}
type Manager struct {
	mu                     sync.Mutex
	Store                  Journal
	Instance, PolicyDigest string
	Permit                 func(context.Context) error
}

func Key(host, root string) string {
	return "lease:" + usage.Digest(host, strings.ToLower(strings.ReplaceAll(root, `\`, "/")))
}
func (m *Manager) save(ctx context.Context, l Lease) error {
	return m.Store.PutUsageJournal(ctx, l.ID, l)
}

// Preserve a known external outcome even if the requesting operation was
// cancelled while its response was in flight. This never starts a mutation.
func (m *Manager) saveKnown(ctx context.Context, l Lease) error {
	settle, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if l.Phase == "revoked" {
		if err := m.Store.PutUsageJournal(settle, "revocation:"+l.ID+":"+strconv.FormatUint(l.Epoch, 10), l); err != nil {
			return err
		}
	}
	return m.save(settle, l)
}

func (m *Manager) allowCreate(ctx context.Context, l *Lease) error {
	err := ctx.Err()
	if err == nil && m.Permit != nil {
		err = m.Permit(ctx)
	}
	if err != nil {
		// The following create has not been sent, so its durable intent can be
		// settled without pretending a lost response has been reconciled.
		l.CreateUnresolved = false
		l.Phase = "releasing"
		if settleErr := m.saveKnown(ctx, *l); settleErr != nil {
			return settleErr
		}
	}
	return err
}

// Acquire is serialized through response settlement. The durable intent precedes
// every create, so a crash cannot make a later attempt blindly create again.
func (m *Manager) Acquire(ctx context.Context, p Port, l Lease, consumer string) (Lease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Permit != nil {
		if err := m.Permit(ctx); err != nil {
			return l, err
		}
	}
	l.ID = Key(l.HostID, l.Root)
	var old Lease
	if err := m.Store.UsageJournal(ctx, l.ID, &old); err == nil {
		if old.ID != l.ID || old.HostID != l.HostID || !samePath(old.Root, l.Root) {
			return old, errors.New("ACCESS_CLEANUP_PENDING")
		}
		if old.Phase != "revoked" {
			if old.Consumers == nil || old.GroupID == "" || old.FolderID == "" {
				return old, errors.New("ACCESS_CLEANUP_PENDING")
			}
			if old.Phase == "active" && old.Instance == m.Instance && old.PolicyDigest == m.PolicyDigest && !old.CreateUnresolved && !old.MutationUnresolved {
				old.Consumers[consumer] = true
				if old.Runs == nil {
					old.Runs = map[string]bool{}
				}
				if old.RunID != "" {
					old.Runs[old.RunID] = true
				}
				if l.RunID != "" {
					old.Runs[l.RunID] = true
				}
				return old, m.save(ctx, old)
			}
			return old, errors.New("ACCESS_CLEANUP_PENDING")
		}
		l.Epoch = old.Epoch + 1
	} else if errors.Is(err, sql.ErrNoRows) {
		l.Epoch = 1
	} else {
		return old, errors.New("ACCESS_CLEANUP_PENDING")
	}
	l.Instance = m.Instance
	l.PolicyDigest = m.PolicyDigest
	// A new epoch owns new remote resources, even when a caller passes the
	// previously revoked lease as its requested host/root configuration.
	l.GroupID, l.FolderID = "", ""
	l.CreateUnresolved, l.MutationUnresolved = false, false
	l.Marker = "SteerRunner usage " + usage.Digest(l.ID, m.Instance, strconv.FormatUint(l.Epoch, 10))[:24]
	l.Consumers = map[string]bool{consumer: true}
	l.Runs = map[string]bool{}
	if l.RunID != "" {
		l.Runs[l.RunID] = true
	}
	l.Phase = "creating_group"
	l.CreateUnresolved = true
	if err := m.save(ctx, l); err != nil {
		return l, err
	}
	if err := m.allowCreate(ctx, &l); err != nil {
		return l, err
	}
	g, err := p.CreateGroup(ctx, l.Marker)
	if err != nil {
		return l, errors.New("ACCESS_CREATE_OUTCOME_UNKNOWN")
	}
	l.GroupID = g.ID
	l.CreateUnresolved = false
	l.Phase = "creating_folder"
	if err = m.saveKnown(ctx, l); err != nil {
		return l, err
	}
	if err = ctx.Err(); err != nil {
		return l, err
	}
	if g.Name != l.Marker || g.ParentPath != nil || g.ParentGroupID != nil || g.ConnectionID != nil {
		return l, errors.New("ACCESS_CLEANUP_PENDING")
	}
	l.CreateUnresolved = true
	if err = m.save(ctx, l); err != nil {
		return l, err
	}
	if err = m.allowCreate(ctx, &l); err != nil {
		return l, err
	}
	f, err := p.CreateFolder(ctx, g.ID, l.Marker, l.Root)
	if err != nil {
		return l, errors.New("ACCESS_CREATE_OUTCOME_UNKNOWN")
	}
	l.FolderID = f.ID
	l.CreateUnresolved = false
	if !ownedFolder(f, l) {
		l.Phase = "releasing"
		_ = m.saveKnown(ctx, l)
		return l, errors.New("ACCESS_CLEANUP_PENDING")
	}
	l.Phase = "active"
	if ctx.Err() != nil {
		l.Phase = "releasing"
	}
	if err = m.saveKnown(ctx, l); err != nil {
		return l, err
	}
	return l, ctx.Err()
}

// ValidateRead rechecks the exact registration immediately before file access.
// It does not turn the generic Orca API into an atomic read-only ACL.
func (m *Manager) ValidateRead(ctx context.Context, p Port, l Lease) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Permit != nil {
		if err := m.Permit(ctx); err != nil {
			return err
		}
	}
	var current Lease
	if err := m.Store.UsageJournal(ctx, l.ID, &current); err != nil {
		return err
	}
	if current.Epoch != l.Epoch || current.Instance != m.Instance || current.PolicyDigest != m.PolicyDigest || current.Phase != "active" || current.CreateUnresolved || current.MutationUnresolved {
		return errors.New("ACCESS_CLEANUP_PENDING")
	}
	groups, err := p.Groups(ctx)
	if err != nil {
		return err
	}
	groupFound := false
	for _, g := range groups {
		if g.ParentGroupID != nil && *g.ParentGroupID == l.GroupID {
			return errors.New("ACCESS_CLEANUP_PENDING")
		}
		if g.ID == l.GroupID {
			if g.Name != l.Marker || g.ParentPath != nil || g.ParentGroupID != nil || g.ConnectionID != nil {
				return errors.New("ACCESS_CLEANUP_PENDING")
			}
			groupFound = true
		}
	}
	if !groupFound {
		return errors.New("ACCESS_CLEANUP_PENDING")
	}
	unused, err := p.GroupUnused(ctx, l.GroupID)
	if err != nil {
		return err
	}
	if !unused {
		return errors.New("ACCESS_CLEANUP_PENDING")
	}
	folders, err := p.Folders(ctx)
	if err != nil {
		return err
	}
	matches := 0
	for _, f := range folders {
		if f.GroupID == l.GroupID && f.ID != l.FolderID {
			return errors.New("ACCESS_CLEANUP_PENDING")
		}
		if f.ID != l.FolderID {
			continue
		}
		matches++
		if !ownedFolder(f, l) {
			return errors.New("ACCESS_CLEANUP_PENDING")
		}
	}
	if matches != 1 {
		return errors.New("ACCESS_CLEANUP_PENDING")
	}
	if m.Permit != nil {
		return m.Permit(ctx)
	}
	return ctx.Err()
}
func samePath(a, b string) bool {
	return strings.EqualFold(strings.ReplaceAll(a, `\`, "/"), strings.ReplaceAll(b, `\`, "/"))
}

// Release checks the original IDs and local epoch again. Safe is an independent
// terminal/context ownership check; no method in this package closes terminals.
func (m *Manager) Release(ctx context.Context, p Port, l Lease, consumer string, safe func(context.Context, Lease) bool) (Lease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var current Lease
	if err := m.Store.UsageJournal(ctx, l.ID, &current); err != nil {
		return l, err
	}
	if current.Epoch != l.Epoch || current.Instance != l.Instance {
		return current, errors.New("ACCESS_CLEANUP_PENDING")
	}
	runtime := l.Runtime
	l = current
	if runtime != "" {
		l.Runtime = runtime
	}
	if l.Phase == "revoked" {
		return l, nil
	}
	if consumer == "" {
		l.Consumers = map[string]bool{}
	} else {
		delete(l.Consumers, consumer)
	}
	if len(l.Consumers) > 0 {
		return l, m.save(ctx, l)
	}
	l.Phase = "releasing"
	if err := m.save(ctx, l); err != nil {
		return l, err
	}
	if err := m.reconcile(ctx, p, &l); err != nil {
		return l, err
	}
	if l.CreateUnresolved {
		return l, errors.New("ACCESS_CREATE_OUTCOME_UNKNOWN")
	}
	if l.MutationUnresolved {
		return l, errors.New("ACCESS_CLEANUP_PENDING")
	}
	folders, err := p.Folders(ctx)
	if err != nil {
		return l, err
	}
	groups, err := p.Groups(ctx)
	if err != nil {
		return l, err
	}
	groupPresent, folderPresent := false, false
	for _, g := range groups {
		if g.ParentGroupID != nil && *g.ParentGroupID == l.GroupID {
			return l, errors.New("ACCESS_CLEANUP_PENDING")
		}
		if g.ID == l.GroupID {
			if g.Name != l.Marker || g.ParentPath != nil || g.ParentGroupID != nil || g.ConnectionID != nil {
				return l, errors.New("ACCESS_CLEANUP_PENDING")
			}
			groupPresent = true
		}
	}
	if groupPresent {
		unused, e := p.GroupUnused(ctx, l.GroupID)
		if e != nil {
			return l, e
		}
		if !unused {
			return l, errors.New("ACCESS_CLEANUP_PENDING")
		}
	}
	for _, f := range folders {
		if f.GroupID == l.GroupID && f.ID != l.FolderID {
			return l, errors.New("ACCESS_CLEANUP_PENDING")
		}
		if f.ID == l.FolderID {
			if !ownedFolder(f, l) {
				return l, errors.New("ACCESS_CLEANUP_PENDING")
			}
			folderPresent = true
		}
	}
	if folderPresent {
		if safe == nil || !safe(ctx, l) {
			return l, errors.New("ACCESS_CLEANUP_PENDING")
		}
		l.MutationUnresolved = true
		if err = m.save(ctx, l); err != nil {
			return l, err
		}
		if err = p.DeleteFolder(ctx, l.FolderID); err != nil {
			return l, err
		}
		l.MutationUnresolved = false
		if err = m.saveKnown(ctx, l); err != nil {
			return l, err
		}
		if err = ctx.Err(); err != nil {
			return l, err
		}
	}
	// Authentication succeeded on the original stable host through this port.
	folders, err = p.Folders(ctx)
	if err != nil {
		return l, err
	}
	for _, f := range folders {
		if f.ID == l.FolderID || f.GroupID == l.GroupID {
			return l, errors.New("ACCESS_CLEANUP_PENDING")
		}
	}
	if l.FolderID != "" && !p.SelectorRevoked(ctx, l.FolderID, l.File) {
		return l, errors.New("ACCESS_CLEANUP_PENDING")
	}
	// Folder deletion is an external operation. Revalidate the group instead
	// of using its name/children/reference facts captured before that call.
	groups, err = p.Groups(ctx)
	if err != nil {
		return l, err
	}
	groupPresent = false
	for _, g := range groups {
		if g.ParentGroupID != nil && *g.ParentGroupID == l.GroupID {
			return l, errors.New("ACCESS_CLEANUP_PENDING")
		}
		if g.ID == l.GroupID {
			if g.Name != l.Marker || g.ParentPath != nil || g.ParentGroupID != nil || g.ConnectionID != nil {
				return l, errors.New("ACCESS_CLEANUP_PENDING")
			}
			groupPresent = true
		}
	}
	if groupPresent {
		unused, e := p.GroupUnused(ctx, l.GroupID)
		if e != nil {
			return l, e
		}
		if !unused {
			return l, errors.New("ACCESS_CLEANUP_PENDING")
		}
		l.MutationUnresolved = true
		if err = m.save(ctx, l); err != nil {
			return l, err
		}
		if err = p.DeleteGroup(ctx, l.GroupID); err != nil {
			return l, err
		}
		l.MutationUnresolved = false
		if err = m.saveKnown(ctx, l); err != nil {
			return l, err
		}
		if err = ctx.Err(); err != nil {
			return l, err
		}
	}
	groups, err = p.Groups(ctx)
	if err != nil {
		return l, err
	}
	for _, g := range groups {
		if g.ID == l.GroupID {
			return l, errors.New("ACCESS_CLEANUP_PENDING")
		}
	}
	l.Phase = "revoked"
	return l, m.saveKnown(ctx, l)
}

func ownedFolder(f native.UsageFolder, l Lease) bool {
	emptyList := func(raw []byte) bool { v := strings.TrimSpace(string(raw)); return v == "" || v == "null" || v == "[]" }
	return f.GroupID == l.GroupID && f.Name == l.Marker && samePath(f.Path, l.Root) && f.ConnectionID == nil && (len(f.LinkedTask) == 0 || string(f.LinkedTask) == "null") && (len(f.LinkedContext) == 0 || string(f.LinkedContext) == "null") && emptyList(f.LinkedItems) && emptyList(f.DiffComments) && !f.Archived && !f.Pinned && f.Comment == ""
}
func (m *Manager) reconcile(ctx context.Context, p Port, l *Lease) error {
	if !l.CreateUnresolved {
		return nil
	}
	groups, err := p.Groups(ctx)
	if err != nil {
		return err
	}
	folders, err := p.Folders(ctx)
	if err != nil {
		return err
	}
	matches := 0
	groupID := ""
	for _, group := range groups {
		if group.Name == l.Marker && group.ParentPath == nil {
			matches++
			groupID = group.ID
		}
	}
	if matches == 1 && l.GroupID == "" {
		l.GroupID = groupID
	}
	matches = 0
	folderID := ""
	for _, folder := range folders {
		if folder.GroupID == l.GroupID && folder.Name == l.Marker && samePath(folder.Path, l.Root) {
			matches++
			folderID = folder.ID
		}
	}
	if matches == 1 && l.FolderID == "" {
		l.FolderID = folderID
	}
	if err = m.save(ctx, *l); err != nil {
		return err
	}
	// Existing Orca RPC has no operation-settlement proof after a lost create
	// response. Listing can identify residual resources, but cannot prove the
	// original operation settled. Keep this intent pending for manual resolution.
	return errors.New("ACCESS_CREATE_OUTCOME_UNKNOWN")
}

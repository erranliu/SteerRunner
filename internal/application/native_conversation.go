package application

import (
	"context"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

var nativeConversationSlots = make(chan struct{}, 2)

func (e *Engine) ReadNativeConversation(target contracts.NativeSessionTarget, cursor string, limit int) contracts.Response {
	reject := func(code, message string, exit int) contracts.Response {
		return contracts.Rejected("", contracts.Fail(exit, code, message))
	}
	if limit == 0 {
		limit = 25
	}
	if target.RunID == "" || target.AttemptID == "" || target.SessionID == "" || target.Generation == "" || limit < 1 || limit > 50 || len(cursor) > 1024 {
		return reject("invalid_arguments", "Invalid native conversation target or page bounds.", 2)
	}
	e.mu.Lock()
	r := e.runs[target.RunID]
	if r == nil {
		e.mu.Unlock()
		return reject("run_not_found", "Original Run was not found.", 5)
	}
	matches := func(s b.LifecycleSnapshot) bool {
		return s.Session.AttemptID == target.AttemptID && s.Session.SessionID == target.SessionID && s.ResourceGeneration == target.Generation
	}
	if !matches(r.Snapshot) {
		e.mu.Unlock()
		return reject("stale_revision", "Original native session changed. Refresh the candidate.", 5)
	}
	if r.Snapshot.Ref.Protocol != b.ProtocolOrcaAgentSessionV1 {
		e.mu.Unlock()
		return reject("unsupported_capability", "Original adapter does not publish native conversation history.", 3)
	}
	// Run revisions are replaced on commit. Pin the original immutable record,
	// then do the native read without the Engine mutex or the worker gate.
	e.mu.Unlock()
	ctx, cancel := context.WithTimeout(e.ctx, 4*time.Second)
	defer cancel()
	select {
	case nativeConversationSlots <- struct{}{}:
		defer func() { <-nativeConversationSlots }()
	case <-ctx.Done():
		return reject("environment_unavailable", "Original conversation is temporarily unavailable.", 4)
	}
	userInputs := make(map[string]bool)
	for _, entry := range r.History {
		if entry.RequestCommandID != "" && (entry.Role == "user" || entry.Role == "answer") {
			userInputs[entry.Text] = true
		}
	}
	page, err := orca.ReadNativeConversation(ctx, e.setup(r, &worker{}), target, cursor, limit, userInputs)
	if orca.IsConversationCursorError(err) {
		return reject("invalid_cursor", "Conversation cursor differs from the original session.", 2)
	}
	if err != nil {
		return reject("environment_unavailable", "Original conversation page could not be verified. Refresh and retry.", 4)
	}
	e.mu.Lock()
	current := e.runs[target.RunID]
	stillBound := current != nil && matches(current.Snapshot)
	e.mu.Unlock()
	if !stillBound {
		return reject("stale_revision", "Original native session changed. Refresh the candidate.", 5)
	}
	page.Partial = page.Partial || r.Snapshot.Session.AttemptNumber > 1
	return contracts.Success("", page)
}

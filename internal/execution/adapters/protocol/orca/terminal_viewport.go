package orca

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// ViewportRPC is local diagnostic data, never input acceptance evidence.
type ViewportRPC struct {
	ID, Phase, Method, RequestID string
	Params                       any
	StartedAt, FinishedAt        time.Time
	Result                       json.RawMessage
	Error                        string
}

type ViewportRecorder func(ViewportRPC) error

// ComposerViewport owns only a temporary layout override on the exact PTY.
// The existing host API checks the unique client ID again when restoring it.
type ComposerViewport struct {
	environment, clientID      string
	process                    b.ProcessIdentity
	previousCols, previousRows int
	record                     ViewportRecorder
	once                       sync.Once
	restored                   json.RawMessage
	restoreErr                 error
}

func viewportCall(ctx context.Context, socket stateConnection, method string, params any, record ViewportRecorder) (json.RawMessage, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	event := ViewportRPC{ID: hex.EncodeToString(nonce[:]), Phase: "intent", Method: method, Params: params, StartedAt: time.Now().UTC()}
	if record != nil {
		if err := record(event); err != nil {
			return nil, err
		}
	}
	id, err := socket.request(ctx, method, params)
	var raw json.RawMessage
	if err == nil {
		raw, err = socket.next(ctx, id)
	}
	event.Phase, event.RequestID, event.Result, event.FinishedAt = "result", id, raw, time.Now().UTC()
	if err != nil {
		event.Error = err.Error()
	}
	if record != nil {
		err = errors.Join(err, record(event))
	}
	return raw, err
}

func viewportBound(ctx context.Context, socket stateConnection, p b.ProcessIdentity, record ViewportRecorder) error {
	raw, err := viewportCall(ctx, socket, "terminal.show", map[string]string{"terminal": p.Terminal}, record)
	if err != nil {
		return err
	}
	var result struct{ Terminal Terminal }
	if json.Unmarshal(raw, &result) != nil {
		return errors.New("original viewport terminal unavailable")
	}
	result.Terminal.RuntimeID = p.RuntimeID // every state envelope fences this runtime
	if err := validateBoundTerminal(result.Terminal, p); err != nil {
		return err
	}
	if p.WorktreeID == "" || result.Terminal.WorktreeID != p.WorktreeID {
		return stateIdentityError("viewport worktree changed")
	}
	return nil
}

// FitBoundComposer does not write, edit or submit input. It requests the host's
// existing temporary fit override so the *unchanged* text can be read exactly.
// A returned lease must be restored even if the resize receipt was lost.
func FitBoundComposer(ctx context.Context, environment string, p b.ProcessIdentity, record ViewportRecorder) (*ComposerViewport, json.RawMessage, error) {
	if p.RuntimeID == "" || p.Terminal == "" || p.Incarnation == "" || p.WorktreeID == "" {
		return nil, nil, stateIdentityError("viewport process identity missing")
	}
	socket, err := openStateConnection(ctx, environment, p.RuntimeID)
	if err != nil {
		return nil, nil, err
	}
	defer socket.close()
	if err := viewportBound(ctx, socket, p, record); err != nil {
		return nil, nil, err
	}
	raw, err := viewportCall(ctx, socket, "terminal.getDisplayMode", map[string]string{"terminal": p.Terminal}, record)
	if err != nil {
		return nil, nil, err
	}
	var mode struct {
		Mode          string
		IsPhoneFitted *bool
	}
	if json.Unmarshal(raw, &mode) != nil || mode.Mode != "auto" || mode.IsPhoneFitted == nil || *mode.IsPhoneFitted {
		return nil, nil, errors.New("original terminal already has a foreign viewport policy; no fit")
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, nil, err
	}
	lease := &ComposerViewport{environment: environment, clientID: "steerrunner-composer-" + hex.EncodeToString(nonce[:]), process: p, record: record}
	raw, err = viewportCall(ctx, socket, "terminal.resizeForClient", map[string]any{"terminal": p.Terminal, "mode": "mobile-fit", "clientId": lease.clientID, "cols": 1024, "rows": 80}, record)
	if err != nil {
		return lease, raw, err
	}
	var fit struct {
		Terminal struct {
			Handle, Mode               string
			Cols, Rows                 int
			PreviousCols, PreviousRows *int
		}
	}
	if json.Unmarshal(raw, &fit) != nil || fit.Terminal.Handle != p.Terminal || fit.Terminal.Mode != "mobile-fit" || fit.Terminal.Cols != 1024 || fit.Terminal.Rows != 80 || fit.Terminal.PreviousCols == nil || fit.Terminal.PreviousRows == nil || *fit.Terminal.PreviousCols <= 0 || *fit.Terminal.PreviousRows <= 0 {
		return lease, raw, errors.New("original composer viewport fit receipt unverified")
	}
	lease.previousCols, lease.previousRows = *fit.Terminal.PreviousCols, *fit.Terminal.PreviousRows
	err = viewportBound(ctx, socket, p, record)
	return lease, raw, err
}

func (v *ComposerViewport) Restore(ctx context.Context) (json.RawMessage, error) {
	v.once.Do(func() {
		socket, err := openStateConnection(ctx, v.environment, v.process.RuntimeID)
		if err != nil {
			v.restoreErr = err
			return
		}
		defer socket.close()
		if err := viewportBound(ctx, socket, v.process, v.record); err != nil {
			v.restoreErr = err
			return
		}
		v.restored, err = viewportCall(ctx, socket, "terminal.resizeForClient", map[string]any{"terminal": v.process.Terminal, "mode": "restore", "clientId": v.clientID}, v.record)
		if err != nil {
			v.restoreErr = err
			return
		}
		var result struct {
			Terminal struct {
				Handle, Mode string
				Cols, Rows   int
			}
		}
		if json.Unmarshal(v.restored, &result) != nil || result.Terminal.Handle != v.process.Terminal || result.Terminal.Mode != "desktop-fit" || result.Terminal.Cols <= 0 || result.Terminal.Rows <= 0 || v.previousCols > 0 && (result.Terminal.Cols != v.previousCols || result.Terminal.Rows != v.previousRows) {
			v.restoreErr = errors.New("original viewport restoration receipt unverified")
			return
		}
		v.restoreErr = viewportBound(ctx, socket, v.process, v.record)
	})
	return v.restored, v.restoreErr
}

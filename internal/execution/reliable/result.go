// Package reliable owns execution policy independently of business workflows.
// Values are transient projections; the actor owns the only durable write.
package reliable

import (
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"strings"
)

var ErrStale = b.ErrTurnStale

func VerifiedContent(verified bool, source, item string) bool {
	return verified && source != "" && item != ""
}

// Result separates native termination, available content and session eligibility.
// Workflow consumes these facts without receiving adapter envelopes or receipts.
type Result struct {
	CommandID, TurnID, SessionID                string
	Succeeded                                   bool
	NativeOutcome, Content, Failure, Diagnostic string
	ContentVerified                             bool
}

func NormalizeResult(session b.SessionContext, evidence b.TurnEvidence) (Result, error) {
	if session.CurrentCommandID != evidence.CommandID || session.CurrentTurnID != evidence.TurnID || session.SessionID != evidence.SessionID || !evidence.Started || !evidence.Completed {
		return Result{}, ErrStale
	}
	outcome := evidence.NativeOutcome
	if outcome == "" {
		outcome = "failed"
		if evidence.Succeeded {
			outcome = "succeeded"
		}
	}
	switch outcome {
	case "succeeded", "failed", "cancelled", "unknown", "unreported":
	default:
		return Result{}, errors.New("invalid normalized native outcome")
	}
	verified := VerifiedContent(evidence.FinalVerified, evidence.FinalSource, evidence.FinalItemID)
	// A missing provider outcome can enter business acquisition only with a
	// complete, normally collected original final. Explicit unknown stays blocked.
	eligible := outcome == "succeeded" || outcome == "unreported" && verified && strings.TrimSpace(evidence.LastMessage) != "" && evidence.Failure == ""
	return Result{CommandID: evidence.CommandID, TurnID: evidence.TurnID, SessionID: evidence.SessionID, Succeeded: evidence.Succeeded && eligible, NativeOutcome: outcome, Content: evidence.LastMessage, Failure: evidence.Failure, Diagnostic: evidence.FinalDiagnostic, ContentVerified: verified}, nil
}

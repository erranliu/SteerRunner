package orcatransport

import (
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

const nativeWaiting = "Native main agent is waiting; no next input was sent"
const nativePromptMissing = "Native original input preview is missing; awaiting original attribution evidence"

type provenanceKind uint8

const (
	provenanceMatched provenanceKind = iota
	provenanceMissing
	provenanceConflict
)

type provenance struct {
	kind       provenanceKind
	diagnostic string
}

// attribution is shared by notifications, independent queries and public
// sampling. Missing fields are not durable contradictions.
func (p NativeProof) attribution(s native.AgentState, turn *Turn, linked bool, input *nativeInput) provenance {
	if s.SessionBoundary || s.RestoredUnconfirmed || s.Prompt != "" && s.Prompt != preview(turn.Prompt) {
		return provenance{provenanceConflict, "Native original completion/input provenance is unverified"}
	}
	if s.ToolOutput {
		return provenance{provenanceMissing, "Native original final body is tool output; awaiting original assistant evidence"}
	}
	if s.Prompt == preview(turn.Prompt) {
		return provenance{}
	}
	if s.Prompt == "" {
		if linked || p.recordedInputMatches(s, turn, input) {
			return provenance{}
		}
		return provenance{provenanceMissing, nativePromptMissing}
	}
	return provenance{provenanceConflict, "Native original completion/input provenance is unverified"}
}

// nativeObservation owns only transient observation coverage and one bounded
// public candidate. NativeProof remains the sole durable execution authority.
type nativeObservation struct {
	identity         b.PublicMessage
	feed             native.StateFeed
	continuous       bool
	turnStartedAt    int64
	lastDigest       string
	lastReceived     time.Time
	lastSourceAt     int64
	pending          *b.PublicMessage
	diagnostic       string
	input            *nativeInput
	inputStamp       int64
	observedRevision int64
}

// A queried input binding is transient and scoped to this exact display stamp.
// Another boundary or reconnect requires a new independent read.

// Orca coalesces tab snapshots. A live connection alone is not boundary
// coverage. The retained native turn-start stamp must survive unchanged from
// the observed original start; absent stamps never grant empty-prompt proof.
func (o *nativeObservation) linked(sample native.StateSample, p NativeProof) bool {
	s := sample.AgentState
	return s.MainAgent != nil && s.MainAgent.State == "done" && o.continuous && o.turnStartedAt > 0 && s.TurnStartedAt == o.turnStartedAt &&
		sample.Kind == native.SampleNotification && trustedState(s) &&
		s.Observation.AuthorityID == p.Authority && s.Observation.Incarnation == p.Incarnation &&
		s.Observation.Revision > p.Revision
}

// A query may fill a missing body under current observer coverage. A separate
// retained-input check in NativeProof.query can recover after reconnect without
// restoring this transient coverage or treating a snapshot as a notification.

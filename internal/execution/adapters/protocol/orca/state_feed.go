package orca

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

type MainAgentState struct {
	State          string `json:"state"`
	Outcome        string `json:"outcome,omitempty"`
	StateStartedAt int64  `json:"stateStartedAt"`
}
type StateObservation struct {
	Origin      string `json:"origin"`
	Kind        string `json:"kind"`
	AuthorityID string `json:"authorityId"`
	Incarnation int64  `json:"incarnation"`
	Revision    int64  `json:"revision"`
	ObservedAt  int64  `json:"observedAt"`
	Boundary    bool   `json:"boundary,omitempty"`
}

// AgentState carries public original-pane facts. The provider's own transcript
// locator is used only with Orca's existing read-only RPC; Runner never opens a
// remote private file. Derived input proof cannot be supplied over the wire.
type AgentState struct {
	State           string            `json:"state"`
	PaneKey         string            `json:"paneKey"`
	AgentType       string            `json:"agentType"`
	Prompt          string            `json:"prompt"`
	UpdatedAt       int64             `json:"updatedAt"`
	StateStartedAt  int64             `json:"stateStartedAt"`
	TurnStartedAt   int64             `json:"turnStartedAt"`
	MainAgent       *MainAgentState   `json:"mainAgent,omitempty"`
	Observation     *StateObservation `json:"observation,omitempty"`
	ProviderSession struct {
		ID             string `json:"id"`
		TranscriptPath string `json:"transcriptPath,omitempty"`
	} `json:"providerSession"`
	NativeInput          *NativeInput      `json:"-"`
	MetadataObservation  *StateObservation `json:"-"`
	LastAssistantMessage string            `json:"lastAssistantMessage,omitempty"`
	Interrupted          bool              `json:"interrupted,omitempty"`
	SessionBoundary      bool              `json:"sessionBoundary,omitempty"`
	RestoredUnconfirmed  bool              `json:"restoredUnconfirmed,omitempty"`
	ToolOutput           bool              `json:"lastAssistantMessageIsToolOutput,omitempty"`
}

var ErrStateIdentityChanged = errors.New("Orca native state identity changed")

func stateIdentityError(message string) error {
	return fmt.Errorf("%w: %s", ErrStateIdentityChanged, message)
}

type SampleKind string

const (
	SampleInitial      SampleKind = "initial"
	SampleNotification SampleKind = "notification"
	SampleQuery        SampleKind = "query"
)

// StateSample is transient receive metadata, never part of the native wire or
// persisted execution proof. Receive time is fixed before buffering.
type StateSample struct {
	AgentState
	ReceivedAt time.Time  `json:"-"`
	Kind       SampleKind `json:"-"`
}

type StateFeed interface {
	Initial() StateSample
	Drain() ([]StateSample, error)
	Query(context.Context) (StateSample, error)
	Close()
}
type StateFeedFactory func(context.Context, string, b.ProcessIdentity) (StateFeed, error)

func DecodeOriginalState(raw json.RawMessage, p b.ProcessIdentity) (AgentState, error) {
	var snapshot struct {
		Worktree string
		Tabs     []struct {
			Terminal, IncarnationID, ParentTabID, LeafID string
			AgentStatus                                  *AgentState
		}
	}
	if json.Unmarshal(raw, &snapshot) != nil || strings.TrimPrefix(snapshot.Worktree, "id:") != p.WorktreeID {
		return AgentState{}, stateIdentityError("snapshot worktree changed")
	}
	for _, tab := range snapshot.Tabs {
		if tab.Terminal != p.Terminal {
			continue
		}
		// Runtime-scoped handles and pane IDs bind the stream. The existing
		// terminal.show checks bind the PTY incarnation around observation;
		// session.tabs does not always include that separate metadata field.
		if tab.IncarnationID != "" && tab.IncarnationID != p.Incarnation {
			return AgentState{}, stateIdentityError("terminal incarnation changed")
		}
		s := AgentState{}
		if tab.AgentStatus != nil {
			s = *tab.AgentStatus
		}
		pane := tab.ParentTabID + ":" + tab.LeafID
		if tab.ParentTabID == "" || tab.LeafID == "" {
			return s, stateIdentityError("original pane identity missing")
		}
		if s.PaneKey != "" && s.PaneKey != pane {
			return s, stateIdentityError("original pane changed")
		}
		s.PaneKey = pane
		return s, nil
	}
	return AgentState{}, stateIdentityError("original terminal missing")
}

var ErrOriginalMainAgentUnavailable = errors.New("Orca query original main agent currently unavailable")

func DecodeOriginalAgent(raw json.RawMessage, worktree, pane string) (AgentState, error) {
	var listing struct {
		Worktrees []struct {
			WorktreeID string
			Agents     []struct {
				AgentState
				ParentPaneKey string
			}
		}
	}
	if json.Unmarshal(raw, &listing) != nil {
		return AgentState{}, errors.New("invalid Orca main-agent query result")
	}
	for _, row := range listing.Worktrees {
		if row.WorktreeID != worktree {
			continue
		}
		for _, agent := range row.Agents {
			if agent.PaneKey != pane {
				continue
			}
			if agent.ParentPaneKey != "" {
				return AgentState{}, stateIdentityError("query selected a child agent")
			}
			return agent.AgentState, nil
		}
		return AgentState{}, ErrOriginalMainAgentUnavailable
	}
	return AgentState{}, errors.New("Orca query original worktree absent from current page")
}

type nativeStateFeed struct {
	environment string
	process     b.ProcessIdentity
	initial     StateSample
	sub         *StateSubscription
	cancel      context.CancelFunc
	once        sync.Once
	mu          sync.Mutex
	updates     []StateSample
	err         error
}

func OpenStateFeed(ctx context.Context, environment string, p b.ProcessIdentity) (StateFeed, error) {
	if environment == "" {
		return openLocalQueryFeed(ctx, p)
	}
	sub, raw, err := SubscribeState(ctx, environment, p.RuntimeID, "id:"+p.WorktreeID)
	if err != nil {
		return nil, err
	}
	initial, err := DecodeOriginalState(raw, p)
	if err != nil {
		sub.Close()
		return nil, err
	}
	watch, cancel := context.WithCancel(context.Background())
	sample := StateSample{AgentState: initial, ReceivedAt: time.Now().UTC(), Kind: SampleInitial}
	f := &nativeStateFeed{environment: environment, process: p, initial: sample, sub: sub, cancel: cancel, updates: []StateSample{sample}}
	go func() {
		defer sub.Close()
		for {
			raw, err := sub.Next(watch)
			var s AgentState
			if err == nil {
				s, err = DecodeOriginalState(raw, p)
			}
			sample := StateSample{AgentState: s, ReceivedAt: time.Now().UTC(), Kind: SampleNotification}
			f.mu.Lock()
			if err != nil {
				if watch.Err() == nil {
					f.err = err
				}
				f.mu.Unlock()
				return
			}
			if len(f.updates) >= 64 {
				f.err = errors.New("Orca native state observation gap")
				f.mu.Unlock()
				return
			}
			f.updates = append(f.updates, sample)
			f.mu.Unlock()
		}
	}()
	return f, nil
}
func (f *nativeStateFeed) Initial() StateSample { return f.initial }
func (f *nativeStateFeed) Drain() ([]StateSample, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	updates := f.updates
	f.updates = nil
	return updates, f.err
}
func (f *nativeStateFeed) Query(ctx context.Context) (StateSample, error) {
	return QueryOriginalSample(ctx, f.environment, f.process, f.initial.PaneKey)
}

func QueryOriginalSample(ctx context.Context, environment string, process b.ProcessIdentity, pane string) (StateSample, error) {
	s, err := QueryOriginalAgent(ctx, environment, process, pane)
	return StateSample{AgentState: s, ReceivedAt: time.Now().UTC(), Kind: SampleQuery}, err
}

// A bounded independent read remains available when the notification stream is
// disconnected. Its target comes from the persisted original execution only.
func QueryOriginalAgent(ctx context.Context, environment string, process b.ProcessIdentity, pane string) (AgentState, error) {
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	raw, err := QueryWorktreeAgents(ctx, environment, process.RuntimeID)
	if err != nil {
		return AgentState{}, err
	}
	s, err := DecodeOriginalAgent(raw, process.WorktreeID, pane)
	if err != nil {
		return s, err
	}
	// This separate read verifies session metadata, not a second completion
	// source: title-projected done from it is never accepted.
	metadata, err := QueryState(ctx, environment, process.RuntimeID, "id:"+process.WorktreeID)
	if err != nil {
		return s, err
	}
	current, err := DecodeOriginalState(metadata, process)
	if err != nil {
		return s, err
	}
	if current.PaneKey != s.PaneKey || s.ProviderSession.ID != "" && current.ProviderSession.ID != "" && current.ProviderSession.ID != s.ProviderSession.ID {
		return s, stateIdentityError("queried original pane/provider metadata changed")
	}
	if current.ProviderSession.ID == "" {
		return s, errors.New("Orca queried original provider metadata currently unavailable")
	}
	s.ProviderSession = current.ProviderSession
	s.SessionBoundary = s.SessionBoundary || current.SessionBoundary
	s.RestoredUnconfirmed = s.RestoredUnconfirmed || current.RestoredUnconfirmed
	s.Interrupted = s.Interrupted || current.Interrupted
	s.ToolOutput = s.ToolOutput || current.ToolOutput
	// This stamp is corroborating metadata only. Never combine it with a
	// different main-agent record observed by the independent query.
	queryStamp := s.TurnStartedAt
	s.TurnStartedAt = 0
	if current.MainAgent != nil && s.MainAgent != nil && current.MainAgent.State == s.MainAgent.State && current.MainAgent.StateStartedAt == s.MainAgent.StateStartedAt &&
		(queryStamp == 0 || queryStamp == current.TurnStartedAt) {
		s.TurnStartedAt = current.TurnStartedAt
		// A display marker cleared for disagreement must not be used as if
		// two independently queried records corroborated the native input.
		if current.AgentType == s.AgentType && *current.MainAgent == *s.MainAgent {
			s.MetadataObservation = current.Observation
		}
	}
	return s, nil
}
func (f *nativeStateFeed) Close() { f.once.Do(func() { f.cancel(); f.sub.Close() }) }

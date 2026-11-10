package application

import (
	"context"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/textanalysis"
	"github.com/erranliu/SteerRunner/internal/usage"
	"github.com/erranliu/SteerRunner/internal/usage/api"
)

// Persist before CLI start, so cancellation/crash cannot turn a dispatched call
// into zero usage. Every physical retry gets a distinct call identity.
func (e *Engine) parserUsage(r *runstate.Record, input workflow.StageInput, sourceID string, config ra.ParserConfig) func() (func(textanalysis.CallObservation), error) {
	return func() (func(textanalysis.CallObservation), error) {
		id := contracts.NewID()
		now := time.Now().UTC()
		event := api.Event{ID: usage.Digest(r.ID, "llm_parser", id), RunID: r.ID, Scope: "runner_internal", CallID: id, CallSource: "llm_parser", CoverageStatus: "pending", SourceResultID: sourceID,
			SourceRef: usage.Digest(r.ID, id), AttemptID: r.Snapshot.Session.AttemptID, Generation: r.Snapshot.ResourceGeneration, CommandID: input.CommandID, StepID: r.Workflow.StepID, Agent: "codex",
			ConfiguredModel: config.Model, ConfiguredReasoningEffort: config.ReasoningEffort, CallTime: &now, TimeBasis: "observed", MetadataBasis: "unknown", ObservedAt: now, Format: "codex-exec-jsonl", ParserVersion: "runner-internal/v1", EvidenceRef: id}
		save := func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			snapshot, err := e.store.GetRunUsage(ctx, r.ID, 0)
			if err != nil {
				return err
			}
			_, err = e.store.CommitUsage(ctx, snapshot, []api.Event{event})
			return err
		}
		if err := save(); err != nil {
			return nil, err
		}
		return func(observation textanalysis.CallObservation) {
			event.SessionID, event.ResponseID = observation.SessionID, observation.ResponseID
			event.CoverageStatus = "uncovered"
			if observation.SessionID != "" && usage.Validate(observation.Tokens) == nil {
				event.CoverageStatus, event.Tokens = "covered", observation.Tokens
			}
			if err := save(); err != nil {
				e.logger.Printf("internal usage result unavailable run=%s call=%s", r.ID, id)
			}
		}, nil
	}
}

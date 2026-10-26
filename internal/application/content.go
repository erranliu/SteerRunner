package application

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

// This is admission into the existing aggregate and lifecycle, not a second
// session runner. Native initialization still goes through reliable.Next.
func freezeContent(ctx context.Context, dir string, r *runstate.Record, attempt, generation string, payload []byte) (*runstate.Record, error) {
	_, docs, access := r.Request.ContentInput()
	path := filepath.Join(dir, "runs", r.ID, "content")
	if r.Request.Generate != nil && r.Request.Generate.Repository != nil {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		source, err := workspace.FreezeContentRepository(ctx, gitHost(), *r.Request.Generate.Repository, path)
		if err != nil {
			return nil, contracts.Fail(4, "directory_unavailable", err.Error())
		}
		r.RepositorySource = source
	} else if access != nil {
		path = filepath.Clean(access.Path)
	} else if err := os.MkdirAll(path, 0700); err != nil {
		return nil, err
	}
	identity, err := workspace.DirectoryIdentity(path)
	if err != nil {
		return nil, contracts.Fail(4, "directory_unavailable", err.Error())
	}
	r.DirectoryIdentity = identity
	r.RepositoryPath = path
	project := "content:" + identity
	profile, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{ImplementationKey: codex.ProfileCodecKey, CodecVersion: codex.ProfileCodecVersion, Purpose: b.ImplementationConfigPurpose, ProjectID: project, ProfileID: r.ID, AcceptedConfigVersion: r.InputDigest}, payload)
	if err != nil {
		return nil, err
	}
	r.Profile = []b.ImplementationDataEnvelope{profile}
	for _, doc := range docs {
		content := []byte(*doc.Content)
		if !utf8.Valid(content) || len(content) > contracts.MaxRequestBytes {
			return nil, contracts.Fail(2, "invalid_input", "Material must be bounded UTF-8 text.")
		}
		fixed := filepath.Join(dir, "inputs", r.ID, filepath.FromSlash(doc.Path))
		if err := writeFixed(fixed, content); err != nil {
			return nil, err
		}
		r.Materials = append(r.Materials, runstate.Material{MaterialView: contracts.MaterialView{Kind: doc.Kind, Path: doc.Path, Digest: reliable.Digest(content), SnapshotPath: fixed, ExecutionLocation: "prompt:" + doc.Path}, Content: string(content)})
	}
	target := b.ExecutionTarget{ID: "local", ExecutionHost: "local"}
	agent := b.AgentSpec{Name: "codex"}
	r.Snapshot = b.LifecycleSnapshot{Ref: b.ExecutionRef{Version: 1, Backend: b.BackendLocal, Protocol: b.ProtocolLocalCodexAppServer, Target: target, Agent: agent, RunID: r.ID, AttemptID: attempt, WorkspacePath: path}, Session: b.SessionContext{ProjectID: project, WorkflowRunID: r.ID, ExecutionID: r.ID, AttemptID: attempt, AttemptNumber: 1, Target: target, Agent: agent}, ConfigurationDigest: r.InputDigest, ResourceGeneration: generation, Workspace: b.WorkspaceIntent{Path: path}}
	r.WorkspaceFact = &b.WorkspaceInspection{Worktree: path, InspectionScope: "read_only_directory", Qualified: true, CheckedAt: time.Now().UTC()}
	p, _ := workflow.Lookup(r.Request.Workflow.ID, r.Request.Workflow.Version)
	var settings contracts.CodexSettings
	_ = json.Unmarshal(r.Request.Execution.Settings, &settings)
	r.Workflow = workflow.Snapshot{Config: &p, StepID: p.Initial, ExecutionModel: &workflow.ModelSettings{Model: settings.Model, ReasoningEffort: settings.ReasoningEffort}}
	instruction, _, _ := r.Request.ContentInput()
	r.History = []contracts.HistoryEntry{{Role: "user", Text: instruction}}
	return r, nil
}

func verifyContentDirectory(r *runstate.Record) error {
	identity, err := workspace.DirectoryIdentity(r.Snapshot.Ref.WorkspacePath)
	if err != nil || identity != r.DirectoryIdentity {
		return errors.Join(err, errors.New("original read-only directory identity changed"))
	}
	if r.Request.Generate != nil && r.Request.Generate.Repository != nil {
		if r.RepositorySource == nil {
			return errors.New("fixed repository source unavailable")
		}
		return workspace.VerifyContentRepository(r.Snapshot.Ref.WorkspacePath, r.RepositorySource)
	}
	return nil
}

func contentPrompt(r *runstate.Record, message string) (string, error) {
	if err := verifyContentDirectory(r); err != nil {
		return "", err
	}
	instruction, _, _ := r.Request.ContentInput()
	if message != "" {
		return message, nil
	}
	materials := []map[string]string{}
	for _, m := range r.Materials {
		materials = append(materials, map[string]string{"path": m.Path, "sha256": m.Digest, "content": m.Content})
	}
	text := "Instruction:\n" + instruction + "\nFrozen submitted materials:\n" + string(canonical(materials))
	if r.RepositorySource != nil {
		text += "\nInvestigate relevant code using read-only tools in the current fixed repository snapshot. Do not read the caller's original checkout or other versions. No writes, commits, PRs, Tasks or DAGs. Document paths are logical relative paths returned as JSON, never files to write. Use caller Roadmap internal codes. Cite relevant source paths and this commit in your documents. Fixed commit: " + r.RepositorySource.Commit + "; tree: " + r.RepositorySource.Tree
	}
	if r.Request.Generate != nil && len(r.Request.Generate.OutputSchema) > 0 {
		text += "\nReturn exactly one JSON value matching this JSON Schema, without Markdown fences:\n" + string(r.Request.Generate.OutputSchema)
	}
	return text, nil
}

// Called under the sole actor lock after native identity normalization. It
// records facts in the same StageResults and advances only the frozen cursor.
func finishContent(r *runstate.Record, input workflow.StageInput, value b.TurnEvidence, result reliable.Result) {
	oversized := r.Request.Generate != nil && (len(result.Content) > contracts.MaxGenerateResultBytes || !utf8.ValidString(result.Content))
	if oversized {
		// Preserve source identity and failure, but never replicate an unbounded
		// body across Run, final, stage and history responses.
		result.Content, value.LastMessage = "", ""
	}
	final := contracts.FinalView{CommandID: input.CommandID, AttemptID: input.AttemptID, SessionID: value.SessionID, TurnID: value.TurnID, NativeOutcome: result.NativeOutcome, Verified: result.ContentVerified, Source: value.FinalSource, ItemID: value.FinalItemID, Text: result.Content, Diagnostic: result.Diagnostic}
	content := &contracts.ContentResult{Status: "failed", Source: final, Diagnostic: result.Failure}
	content.RepositorySource = r.RepositorySource
	if result.Succeeded && result.ContentVerified && result.NativeOutcome == "succeeded" && result.Failure == "" {
		content.Status, content.Text = "succeeded", result.Content
		if oversized {
			content.Status, content.Text, content.Diagnostic = "failed", "", "generate_result_limit_exceeded_or_invalid_utf8"
		} else if r.RepositorySource != nil && verifyContentDirectory(r) != nil {
			content.Status, content.Text, content.Diagnostic = "failed", "", "repository_snapshot_changed"
		} else if r.Request.Generate != nil && len(r.Request.Generate.OutputSchema) > 0 {
			data, err := ra.ValidateOutput(r.Request.Generate.OutputSchema, result.Content)
			if err != nil {
				content.Status, content.Text, content.Diagnostic = "failed", "", err.Error()
			} else {
				content.Text, content.JSON = "", data
			}
		}
	} else if result.NativeOutcome == "cancelled" {
		content.Status = "cancelled"
	} else if !result.ContentVerified && result.Succeeded {
		content.Diagnostic = "complete_final_unavailable"
	}
	r.Workflow.StageResults = append(r.Workflow.StageResults, workflow.StageResult{SessionID: value.SessionID, CommandID: input.CommandID, AttemptID: input.AttemptID, StepID: input.StepID, InputVersion: input.Version, TurnID: value.TurnID, Outcome: result.NativeOutcome, Succeeded: result.Succeeded, FinalVerified: result.ContentVerified, Final: result.Content, Failure: result.Failure, FinalDiagnostic: result.Diagnostic})
	r.Result, r.LastFinal, r.ContentResult = &value, &final, content
	r.History = append(r.History, contracts.HistoryEntry{CommandID: input.CommandID, SessionID: value.SessionID, TurnID: value.TurnID, Role: "assistant", Text: result.Content, Result: content})
	for i := range r.History {
		if r.History[i].CommandID == input.CommandID {
			r.History[i].SessionID, r.History[i].TurnID = value.SessionID, value.TurnID
		}
	}
	r.Snapshot.Session.LastStoppedTurnID = value.TurnID
	r.Snapshot.PendingTurn, r.NextStage = nil, nil
	if r.PendingAnswer != nil {
		r.CommandStates[r.PendingAnswer.CommandID] = "failed"
		r.PendingAnswer = nil
	}
	r.Question, r.Snapshot.QuestionRoute, r.Snapshot.PendingQuestions = nil, nil, nil
	if r.PendingInterrupt != "" {
		r.CommandStates[r.PendingInterrupt] = "completed"
		r.PendingInterrupt = ""
	}
	r.Diagnostic = content.Diagnostic
	state, _ := r.Workflow.State(input.StepID)
	if state.Next == "closed" {
		closeContent(r, "generate:"+input.CommandID)
	} else if !r.Closed {
		r.Workflow.StepID, r.Status, r.Phase = state.Next, "waiting_input", "turn_completed"
	}
}

func closeContent(r *runstate.Record, command string) {
	if r.PendingFeedback != nil {
		r.CommandStates[r.PendingFeedback.CommandID] = "failed"
		r.PendingFeedback = nil
	}
	for _, pending := range []string{r.PendingInterrupt, answerCommand(r.PendingAnswer)} {
		if pending == "" {
			continue
		}
		status := "failed"
		for _, intent := range r.Intents {
			if intent.Request.Operation.IdempotencyKey == pending || intent.Request.Input.Feedback != nil && intent.Request.Input.Feedback.DecisionID == pending {
				status = "unknown"
			}
		}
		r.CommandStates[pending] = status
	}
	if turn := r.Snapshot.PendingTurn; turn != nil {
		status := "failed"
		for _, intent := range r.Intents {
			if intent.Request.Input.Turn != nil && intent.Request.Input.Turn.CommandID == turn.CommandID && intent.Request.Operation.Action == string(b.LifecycleStartTurn) {
				status = "unknown"
			}
		}
		requestCommand := r.SubmitCommandID
		if input := workflow.Input(r.Workflow, turn.CommandID); input != nil && input.FeedbackDecisionID != "" {
			requestCommand = input.FeedbackDecisionID
		}
		if requestCommand != "" {
			r.CommandStates[requestCommand] = status
		}
	}
	r.Closed, r.Allowed, r.ResumeRequired = true, false, false
	r.Workflow.StepID, r.PendingStop, r.StopStatus, r.Phase = "closed", command, "pending", "closing"
	r.Status = "settling"
}

func answerCommand(answer *runstate.Answer) string {
	if answer != nil {
		return answer.CommandID
	}
	return ""
}

func settleContentStatus(r *runstate.Record) {
	if !workflow.Content(r.Request.Workflow.ID) || !r.Closed {
		return
	}
	r.Status, r.Phase = "closed", "closed"
	r.Question, r.Snapshot.QuestionRoute, r.Snapshot.PendingQuestions = nil, nil, nil
	if r.Request.Generate != nil && r.ContentResult != nil {
		r.Status = r.ContentResult.Status
		if r.Status == "succeeded" {
			r.Status = "completed"
		}
	}
}

func contentHistory(r *runstate.Record, after string, limit int) contracts.Response {
	start := 0
	if after != "" {
		v, ok := strings.CutPrefix(after, r.ID+":")
		n, err := strconv.Atoi(v)
		if !ok || err != nil || n < 0 || n > len(r.History) {
			return contracts.Rejected("", contracts.Fail(2, "invalid_cursor", "History cursor does not belong to this Run."))
		}
		start = n
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	end := min(start+limit, len(r.History))
	return contracts.Success("", map[string]any{"run": r.View(), "run_id": r.ID, "entries": r.History[start:end], "next_cursor": r.ID + ":" + strconv.Itoa(end), "has_more": end < len(r.History)})
}

// The interrupt intent uses the existing stop_observation native action, whose
// Codex semantics are turn/interrupt. Whole-run stop remains stop_execution.
func (e *Engine) interruptContent(id string, w *worker, p b.LifecyclePort, r *runstate.Record) bool {
	for _, intent := range r.Intents {
		if intent.Request.Operation.IdempotencyKey == r.PendingInterrupt {
			return false
		}
	}
	i, err := e.saveIntent(id, r, b.LifecycleCandidate{Action: b.LifecycleStopObservation, Input: b.LifecycleInput{Stop: &b.StopInput{TurnID: r.Snapshot.Session.CurrentTurnID}}})
	if err != nil {
		return false
	}
	return e.invoke(id, w, p, i, false)
}

func (e *Engine) contentMessage(id string, w *worker, p b.LifecyclePort, r *runstate.Record, answer *runstate.Answer) bool {
	if err := verifyContentDirectory(r); err != nil {
		e.failControl(id, answer.CommandID, err)
		return false
	}

	if err := e.qualifyContinuation(r, w, p, true, func(b.LifecycleRecoveryEvidence, error) error {
		return errors.New("original content session is not verifiably idle")
	}); err != nil {
		e.failControl(id, answer.CommandID, err)
		return false
	}
	if err := queueInput(r, workflow.StageInput{StepID: "execute", FeedbackDecisionID: answer.CommandID}, answer.Text); err != nil {
		e.failControl(id, answer.CommandID, err)
		return false
	}
	r.PendingFeedback = nil
	r.CommandStates[answer.CommandID] = "running"
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.runs[id].Revision != r.Revision {
		return false
	}
	return e.commitLocked(r, nil, "message_frozen") == nil
}

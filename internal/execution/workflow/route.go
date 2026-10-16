package workflow

import (
	"fmt"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
)

// Transition declares business changes. It never sends input or saves records.
type Transition struct {
	Next                                                    *StageInput
	Status, Reason, Outcome, PRCandidateURL                 string
	Report                                                  *Report
	ReportRetry                                             uint64
	Repair                                                  bool
	AcceptResult, RecoverOriginal, Publish, ExpireQuestions bool
}

func PublicationSourceSucceeded(w Snapshot, input StageInput) bool {
	if input.StepID != StepPublishReport {
		return input.StepID == StepPublish
	}
	source, original := Result(w, input.SourceCommandID), Input(w, input.SourceCommandID)
	return source != nil && original != nil && source.StepID == StepPublish && original.StepID == StepPublish && source.TurnID == input.SourceTurnID && source.Succeeded && source.Failure == "" && source.FinalVerified
}

// Route is the single owner of development/v3 result interpretation and retry.
// ReportRetry is a business format correction, never a transport retry.
func Route(w Snapshot, input StageInput, result reliable.Result, reportPolicy string) Transition {
	t := Transition{ReportRetry: w.ReportRetry, AcceptResult: true}
	if AuditStep(input.StepID) && (w.Config == nil || !AuditSource(w, input)) || PublicationStep(input.StepID) && !AuditsPassed(w) {
		t.Status, t.Reason = "blocked", "audit source or publication gate is invalid"
		return t
	}
	failedCore := !result.Succeeded && (input.StepID == StepImplement || input.StepID == StepReport || AuditStep(input.StepID))
	if !failedCore && input.StepID != StepImplement && !result.ContentVerified {
		t.AcceptResult = false
		t.Status = "reconciling"
		t.Reason = "original stopped turn lacks verified final provenance: " + result.Diagnostic
		return t
	}
	if failedCore {
		t.RecoverOriginal = true
		t.Outcome = "blocked"
		t.Reason = "Original turn failed or was interrupted; continue the original session to verify current work: " + result.Failure
		return t
	}
	if reportPolicy == "" {
		reportPolicy = "frozen_native_permissions; business_read_only_is_prompt_policy"
	}
	if input.StepID == StepImplement {
		t.Next = &StageInput{StepID: StepReport, SourceCommandID: input.CommandID, SourceTurnID: result.TurnID, ReportToolPolicy: reportPolicy}
		return t
	}
	report, err := DecodeReport(result.Content, input.StepID)
	if err == nil && PublicationStep(input.StepID) {
		t.PRCandidateURL = report.Outputs.PRURL
	}
	if err == nil && report.Outcome == "blocked" && report.Blocker.Category == "unknown_external" {
		t.Report = &report
		t.Outcome = report.Outcome
		t.Status = "reconciling"
		t.Reason = blockerReason(report.Blocker)
		return t
	}
	if err != nil || !result.Succeeded && report.Outcome == "done" {
		t.Reason = "result_invalid: " + result.Failure
		if err != nil {
			t.Reason += "; " + err.Error()
		}
		if err != nil && input.StepID == StepPublish {
			t.Next = &StageInput{StepID: StepPublishReport, SourceCommandID: input.CommandID, SourceTurnID: result.TurnID, ReportToolPolicy: reportPolicy}
			return t
		}
		if input.StepID == StepReport && w.ReportRetry == 0 {
			t.ReportRetry = 1
			t.Next = &StageInput{StepID: input.StepID, SourceCommandID: input.SourceCommandID, SourceTurnID: input.SourceTurnID, ReportToolPolicy: input.ReportToolPolicy}
			return t
		}
		t.Status = "blocked"
		return t
	}
	t.Report = &report
	t.Outcome = report.Outcome
	switch report.Outcome {
	case "needs_input":
		t.Status = "waiting_input"
		t.Reason = report.Question
		return t
	case "blocked":
		t.Status = "blocked"
		t.Reason = blockerReason(report.Blocker)
		return t
	}
	if input.StepID == StepReport {
		source := Result(w, input.SourceCommandID)
		if source == nil || source.TurnID != input.SourceTurnID || !source.Succeeded || source.Failure != "" {
			t.Status = "blocked"
			t.Reason = "source execution failed; report generated, original failure retained"
			return t
		}
		t.ExpireQuestions = true
		if w.Config != nil && len(w.Config.AuditSteps) > 0 {
			t.Next = &StageInput{StepID: w.Config.AuditSteps[0], SourceCommandID: input.CommandID, SourceTurnID: result.TurnID}
		} else {
			t.Next = &StageInput{StepID: StepPublish}
		}
		return t
	}
	if AuditStep(input.StepID) {
		if report.Verdict == "changes_requested" {
			if w.RepairCount >= w.Config.MaximumRepairCycles {
				t.Status, t.Reason = "blocked", "audit_repair_limit"
				return t
			}
			findings := []Finding{}
			for _, finding := range report.Findings {
				if finding.Severity != "P3" {
					findings = append(findings, finding)
				}
			}
			t.Repair = true
			t.Next = &StageInput{StepID: StepImplement, SourceCommandID: input.CommandID, SourceTurnID: result.TurnID, RepairFindings: findings}
			return t
		}
		current := w
		current.Audits = append(append([]AuditRecord(nil), w.Audits...), AuditRecord{StepID: input.StepID, SourceCommandID: input.SourceCommandID, Verdict: report.Verdict})
		for index, step := range w.Config.AuditSteps {
			if step != input.StepID {
				continue
			}
			if index+1 < len(w.Config.AuditSteps) {
				t.Next = &StageInput{StepID: w.Config.AuditSteps[index+1], SourceCommandID: input.SourceCommandID, SourceTurnID: input.SourceTurnID}
				return t
			}
			if !AuditsPassed(current) {
				t.Status, t.Reason = "blocked", "audit sequence is incomplete"
				return t
			}
			t.Next = &StageInput{StepID: StepPublish}
			return t
		}
		t.Status, t.Reason = "blocked", "audit step is not in frozen workflow"
		return t
	}
	if input.StepID == StepPublishReport && !PublicationSourceSucceeded(w, input) {
		t.Status = "reconciling"
		t.Reason = "source publication failed; report generated, original failure retained"
		return t
	}
	t.Publish = true
	return t
}
func blockerReason(b *Blocker) string {
	return fmt.Sprintf("%s: %s; %s; known=%s; unknown=%s", b.Category, b.Action, b.Error, b.Known, b.Unknown)
}

package workflow

import ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"

type Profile struct {
	ID                 string           `json:"id"`
	Version            int              `json:"version"`
	Description        string           `json:"description"`
	SupportedAdapters  []string         `json:"supported_adapters"`
	AuditModelRequired bool             `json:"audit_model_required"`
	PromptSteps        []string         `json:"prompt_steps"`
	AuditSteps         []string         `json:"audit_steps,omitempty"`
	States             map[string]State `json:"states"`
	Initial            string           `json:"initial"`
}
type ModelSettings struct {
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

const AuditedDevelopment = "development-audited"

func Lookup(id string, version int) (Profile, bool) {
	if version == 1 && (id == "conversation" || id == "generate") {
		return contentProfile(id), true
	}
	if version != 2 {
		return Profile{}, false
	}
	p := Profile{ID: id, Version: version, Initial: StepImplement, SupportedAdapters: []string{"codex-app-server", "orca-terminal"}, PromptSteps: []string{StepImplement, StepReport, "publish", StepPublishReport}}
	switch id {
	case "development":
		p.Description = "Implement and publish a verified PR."
	case AuditedDevelopment:
		p.Description = "Implement, audit and repair code once, then publish the verified revision."
		p.SupportedAdapters = []string{"codex-app-server", "orca-terminal"}
		p.AuditModelRequired = true
		p.AuditSteps = []string{StepAuditCode}
		p.PromptSteps = append(p.PromptSteps, "audit.code", StepAuditReport)
	default:
		return Profile{}, false
	}
	p.States = developmentStates(p.AuditModelRequired)
	return p, true
}
func Catalog() []Profile {
	p, _ := Lookup("development", 2)
	a, _ := Lookup(AuditedDevelopment, 2)
	c, _ := Lookup("conversation", 1)
	g, _ := Lookup("generate", 1)
	return []Profile{p, a, c, g}
}
func AuditStep(step string) bool {
	return step == StepAuditCode || step == StepAuditReport || step == StepAuditParse || step == StepAuditInput || step == "audit.code"
}

// WorkspaceSource is an observation of the original native business turn. No
// later report, current clean HEAD, or model statement can manufacture it.
type WorkspaceSource struct {
	Repository string `json:"repository"`
	Worktree   string `json:"worktree"`
	Branch     string `json:"branch"`
	Base       string `json:"base"`
	Head       string `json:"head"`
	Tree       string `json:"tree"`
	Dirty      bool   `json:"dirty"`
	Qualified  bool   `json:"qualified"`
	Diagnostic string `json:"diagnostic,omitempty"`
}
type AuditRecord struct {
	StepID          string              `json:"step"`
	SourceResultID  string              `json:"source_result_id"`
	Source          ra.Source           `json:"implementation_source"`
	ResultID        string              `json:"result_id"`
	CommandID       string              `json:"command_id"`
	TurnID          string              `json:"turn_id"`
	PreHead         string              `json:"pre_head,omitempty"`
	PostHead        string              `json:"post_head,omitempty"`
	FixedCount      *int                `json:"fixed_count,omitempty"`
	FixedIssues     []ra.FixedIssue     `json:"fixed_issues"`
	RemainingIssues []ra.RemainingIssue `json:"remaining_issues"`
	Failure         string              `json:"failure,omitempty"`
}

func Acquired(w Snapshot, id string) *ra.Result {
	for i := range w.Results {
		if w.Results[i].ID == id {
			return &w.Results[i]
		}
	}
	return nil
}
func AuditSource(w Snapshot, input StageInput) bool {
	source := Acquired(w, input.ImplementationResultID)
	return source != nil && source.ID == w.ImplementationResultID && source.Status == "satisfied" && source.Value != nil && source.Value.Outcome == "done" && source.Source.CommandID == input.SourceCommandID && source.Source.TurnID == input.SourceTurnID && source.Source.InputVersion == input.SourceVersion
}
func AuditsPassed(w Snapshot) bool {
	if !w.RequiresWorkspace() {
		return true
	}
	a := Acquired(w, w.AuditResultID)
	i := Acquired(w, w.ImplementationResultID)
	if a == nil || i == nil || a.Status != "satisfied" || a.Value == nil || a.Value.Outcome != "done" {
		return false
	}
	for _, record := range w.Audits {
		if record.ResultID == a.ID && record.SourceResultID == i.ID && record.Failure == "" && record.PreHead != "" && record.PostHead != "" {
			return true
		}
	}
	return false
}

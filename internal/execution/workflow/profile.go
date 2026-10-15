package workflow

// Profile describes built-in choices, not an executable workflow graph.
type Profile struct {
	ID                  string   `json:"id"`
	Version             int      `json:"version"`
	Description         string   `json:"description"`
	SupportedAdapters   []string `json:"supported_adapters"`
	AuditModelRequired  bool     `json:"audit_model_required"`
	PromptSteps         []string `json:"prompt_steps"`
	AuditSteps          []string `json:"audit_steps,omitempty"`
	MaximumRepairCycles uint64   `json:"maximum_repair_cycles"`
}

type ModelSettings struct {
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

const (
	AuditedDevelopment = "development-audited"
	StepAuditSpec      = "audit.spec"
	StepAuditCode      = "audit.code"
)

func Lookup(id string, version int) (Profile, bool) {
	p := Profile{ID: id, Version: version, SupportedAdapters: []string{"codex-app-server", "orca-terminal"}, PromptSteps: []string{StepImplement, StepReport, StepPublish, StepPublishReport}}
	if version != 1 {
		return Profile{}, false
	}
	switch id {
	case "development":
		p.Description = "Implement, report and publish a verified PR."
	case AuditedDevelopment:
		p.Description = "Implement, audit Spec and code, repair up to twice, then publish."
		p.SupportedAdapters = []string{"codex-app-server"}
		p.AuditModelRequired = true
		p.AuditSteps = []string{StepAuditSpec, StepAuditCode}
		p.PromptSteps = append(p.PromptSteps, p.AuditSteps...)
		p.MaximumRepairCycles = 2
	default:
		return Profile{}, false
	}
	return p, true
}

func Catalog() []Profile {
	plain, _ := Lookup("development", 1)
	audited, _ := Lookup(AuditedDevelopment, 1)
	return []Profile{plain, audited}
}

func AuditStep(step string) bool { return step == StepAuditSpec || step == StepAuditCode }

type Finding struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line,omitempty"`
}

type AuditRecord struct {
	StepID          string    `json:"step"`
	SourceCommandID string    `json:"source_report_command_id"`
	CommandID       string    `json:"command_id"`
	TurnID          string    `json:"turn_id"`
	Verdict         string    `json:"verdict,omitempty"`
	Findings        []Finding `json:"findings,omitempty"`
	Failure         string    `json:"failure,omitempty"`
}

// AuditSource validates the current implementation report and its execution.
func AuditSource(w Snapshot, input StageInput) bool {
	if input.SourceCommandID == "" || input.SourceCommandID != w.AuditReportCommandID {
		return false
	}
	source, original := Result(w, input.SourceCommandID), Input(w, input.SourceCommandID)
	if source == nil || original == nil || source.StepID != StepReport || original.StepID != StepReport || source.TurnID != input.SourceTurnID || !source.Succeeded || !source.FinalVerified || source.Failure != "" {
		return false
	}
	report, err := DecodeReport(source.Final, StepReport)
	execution := Result(w, original.SourceCommandID)
	return err == nil && report.Outcome == "done" && execution != nil && execution.TurnID == original.SourceTurnID && execution.Succeeded && execution.Failure == ""
}

func AuditsPassed(w Snapshot) bool {
	if w.Config == nil {
		return true
	}
	for _, step := range w.Config.AuditSteps {
		found := false
		for _, a := range w.Audits {
			if a.StepID == step && a.SourceCommandID == w.AuditReportCommandID && a.Verdict == "pass" && a.Failure == "" {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return w.AuditReportCommandID != ""
}

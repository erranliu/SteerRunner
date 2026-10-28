package workflow

import ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"

const AuditPrompt = "审计修复 P0–P2；必要验证后提交，无改动沿用提交。\n"
const AuditReportPrompt = "仅报告原审计；不补做或编造。"

// State is a frozen built-in choice of existing capabilities, not a user DSL.
// StepID in Snapshot is the only active business cursor; States are immutable.
type State struct {
	PolicySlots          []string `json:"policy_slots"`
	ReportLimit          int      `json:"report_limit,omitempty"`
	ResultInstruction    string   `json:"result_instruction,omitempty"`
	ManualImplementation string   `json:"manual_implementation,omitempty"`
	CommitWork           bool     `json:"commit_work,omitempty"`
	Kind                 string   `json:"kind"`
	Contract             string   `json:"contract,omitempty"`
	PromptKey            string   `json:"prompt_key,omitempty"`
	Prompt               string   `json:"prompt,omitempty"`
	ModelRole            string   `json:"model_role,omitempty"`
	SuccessOutput        string   `json:"success_output,omitempty"`
	Next                 string   `json:"next,omitempty"`
	Verify               string   `json:"verify,omitempty"`
	Report               string   `json:"report,omitempty"`
	Parse                string   `json:"parse,omitempty"`
	Wait                 string   `json:"wait,omitempty"`
	Continue             string   `json:"continue,omitempty"`
	Workspace            string   `json:"workspace,omitempty"`
}

func developmentStates(audited bool) map[string]State {
	implement := State{PolicySlots: []string{"git", "command", "testing"}, Kind: "execute", Contract: "implementation", PromptKey: StepImplement, Prompt: ImplementPrompt, ModelRole: "execution", SuccessOutput: "实施及必要验证已完成。", Next: StepPublish, Report: StepReport, ReportLimit: 1, Wait: StepImplementInput}
	publish := State{PolicySlots: []string{"git", "command"}, Kind: "execute", Contract: "publication", PromptKey: "publish", Prompt: publishPrompt, ModelRole: "execution", SuccessOutput: "本次发布已完成。", Next: StepPRReady, Verify: StepPublishVerify, Report: StepPublishReport, ReportLimit: 1}
	if audited {
		implement.Next = StepAuditCode
		implement.Workspace = "implementation"
		implement.CommitWork = true
		publish.Workspace = "publication"
	}
	states := map[string]State{
		StepImplement:      implement,
		StepReport:         {Kind: "report", Contract: implement.Contract, PromptKey: StepReport, Prompt: reportPrompt, ModelRole: "execution", Next: implement.Next, Wait: implement.Wait, Workspace: implement.Workspace},
		StepImplementInput: {Kind: "input", Continue: StepImplement},
		StepPublish:        publish,
		StepPublishReport:  {Kind: "report", Contract: publish.Contract, PromptKey: StepPublishReport, Prompt: PublishReportPrompt, ModelRole: "execution", Next: publish.Next, Verify: publish.Verify, Workspace: publish.Workspace},
		StepPublishVerify:  {Kind: "verify", Contract: "publication", Next: StepPRReady, Workspace: publish.Workspace},
		StepPRReady:        {Kind: "milestone"},
	}
	if audited {
		states[StepAuditCode] = State{PolicySlots: []string{"git", "command", "testing"}, Kind: "execute", Contract: "audit", PromptKey: "audit.code", Prompt: AuditPrompt + ra.ResultInstruction("audit"), ModelRole: "audit", Next: StepPublish, Report: StepAuditReport, ReportLimit: 1, Parse: StepAuditParse, Wait: StepAuditInput, Workspace: "audit"}
		states[StepAuditParse] = State{Kind: "parse", Contract: "audit", Prompt: "仅整理已有代码审计与修复结果；不得重审、测试、修复或发布。缺失信息如实表达，不由沉默推断成功或零修复。", Next: StepPublish, Report: StepAuditReport, ReportLimit: 1, Wait: StepAuditInput, Workspace: "audit"}
		states[StepAuditReport] = State{Kind: "report", Contract: "audit", PromptKey: StepAuditReport, Prompt: AuditReportPrompt, ModelRole: "audit", Next: StepPublish, Wait: StepAuditInput, Workspace: "audit"}
		states[StepAuditInput] = State{Kind: "input", ManualImplementation: StepImplement}
	}
	for key, state := range states {
		if state.PolicySlots == nil {
			state.PolicySlots = []string{}
		}
		if state.Kind == "report" {
			state.ResultInstruction = ra.ResultInstruction(state.Contract)
		}
		states[key] = state
	}
	return states
}

func (w Snapshot) State(step string) (State, bool) {
	if w.Config == nil {
		return State{}, false
	}
	s, ok := w.Config.States[step]
	return s, ok
}

func (w Snapshot) CurrentState() (State, bool) { return w.State(w.StepID) }

func (w Snapshot) RequiresWorkspace() bool {
	for _, s := range w.ConfigStates() {
		if s.Workspace != "" {
			return true
		}
	}
	return false
}
func (w Snapshot) ConfigStates() map[string]State {
	if w.Config == nil {
		return nil
	}
	return w.Config.States
}

package workflow

const ProgressInstruction = "STEERLINE_PROGRESS_FILE: raw<=65536B; lock/read-merge then same-dir temp+atomic replace. Runner preserves bytes, not semantics."
const ResultFileInstruction = "STEERLINE_RESULT_FILE: raw<=65536B. When asked to write, coordinate writers and use same-dir atomic replace. Sealed once after execution/producers stop; not a success signal."
const ResultFileSnapshotInstruction = "STEERLINE_RESULT_FILE: independent raw<=65536B; coordinate writers, same-dir atomic replace. Runner takes one snapshot at original business end or explicit stop. Finish writing before the command returns; not a success signal."
const DevelopmentResultFileSnapshotInstruction = "STEERLINE_RESULT_FILE: independent raw<=65536B; coordinate writers, same-dir atomic replace. Runner takes one snapshot at the original explicit stop. Finish writing before the command returns; not a success signal."
const ContinueProgressInstruction = "progress: same session rules."
const ContinueResultInstruction = "result: same session rules."
const PlanDrivenOrcaFileEnvironment = "文件绑定的键为环境变量名，路径相对 worktree 根目录。"
const ContentRepositoryInstruction = "Investigate only fixed read-only snapshot; no writes/commits/PRs/Tasks/DAGs. Return document paths as logical JSON, use caller Roadmap codes; cite source paths and commit/tree: "
const ContentSchemaInstruction = "Return one JSON value matching schema, no fences:\n"
const ContentTaskLabel = "Task:\n"
const ContentMaterialsLabel = "\nMaterials: ["
const PublicationLabel = "\nPR target: "

// InputContract is discovery data for the existing built-ins, not a DSL.
func InputContract() any {
	return map[string]any{
		"orca_file_environment": map[string]any{"sources": []string{"orca.file_environment", "orca.file_bindings"}, "text": OrcaFileEnvironmentInstruction, "plan_driven_text": PlanDrivenOrcaFileEnvironment, "when": "structured Orca with enabled result/progress files; bindings contain original worktree-relative channel paths"},
		"project_policies":      map[string]any{"slots": PolicySlots(), "read_template": PolicyReadTemplate, "when": "only non-plan-driven states and state.policy_slots with exists=true in the frozen project commit; Runner input budget; no inline content"},
		"body":                  "state.prompt; explicit prompt_overrides replaces only the original body, including audit.execute's default format",
		"content":               map[string]any{"task_label": ContentTaskLabel, "material_label": ContentMaterialsLabel, "material_frame": "JSON array: inline content or frozen file location, path and sha256; file bodies are not expanded", "repository": ContentRepositoryInstruction, "schema": ContentSchemaInstruction},
		"facts_label":           FactsLabel,
		"materials":             "implementation execute: Spec/Plan in the original worktree; non-plan-driven states also render distinct reference paths",
		"facts":                 "report source=original command_id, optional failure; exact turn/version/digest and audit/publication prerequisites stay program-verified, never internal ledgers",
		"result_format":         "implementation execute and reports keep the fixed state.result_instruction; audit execute format remains part of overridable body",
		"commit":                CommitInstruction, "success": SuccessLabel, "feedback": FeedbackLabel,
		"publication": map[string]any{"label": PublicationLabel, "slots": "execute only: provider/repository/base_url/remote/base and optional draft; draft adds the fixed initial draft creation instruction; no provider query or authentication configuration"},
		"progress":    map[string]any{"text": ProgressInstruction, "continue": ContinueProgressInstruction, "when": "enabled non-plan-driven implementation/audit execute and execute/1; reuse rule only if already supplied in this original session"},
		"result_file": map[string]any{"text": ResultFileInstruction, "remote_orca_execute_snapshot": ResultFileSnapshotInstruction, "legacy_orca_development_snapshot": DevelopmentResultFileSnapshotInstruction, "continue": ContinueResultInstruction, "when": "enabled non-plan-driven implementation/audit execute and execute/1; snapshot instructions apply only to their matching frozen legacy profiles; reuse rule only if already supplied in this original session"},
	}
}

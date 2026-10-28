package workflow

const ProgressInstruction = "STEERLINE_PROGRESS_FILE: raw<=65536B; lock/read-merge then same-dir temp+atomic replace. Runner preserves bytes, not semantics."
const ResultFileInstruction = "STEERLINE_RESULT_FILE: independent raw<=65536B; coordinate writers, same-dir atomic replace. Sealed once after execution/producers stop, never turn end/pr_ready; not a success signal."
const ContinueProgressInstruction = "progress: same session rules."
const ContinueResultInstruction = "result: same session rules."
const ContentRepositoryInstruction = "Investigate only fixed read-only snapshot; no writes/commits/PRs/Tasks/DAGs. Return document paths as logical JSON, use caller Roadmap codes; cite source paths and commit/tree: "
const ContentSchemaInstruction = "Return one JSON value matching schema, no fences:\n"
const ContentTaskLabel = "Task:\n"
const ContentMaterialsLabel = "\nMaterials: ["
const PublicationLabel = "\nPR target: "

// InputContract is discovery data for the existing built-ins, not a DSL.
func InputContract() any {
	return map[string]any{
		"body":          "state.prompt; explicit prompt_overrides replaces only the original body, including audit.execute's default format",
		"content":       map[string]any{"task_label": ContentTaskLabel, "material_label": ContentMaterialsLabel, "material_frame": "JSON array: content (escaped caller text), path, sha256 (Runner framing and facts)", "repository": ContentRepositoryInstruction, "schema": ContentSchemaInstruction},
		"facts_label":   FactsLabel,
		"materials":     "implementation execute: Spec/Plan and distinct reference paths in the original worktree",
		"facts":         "report source=original command_id, optional failure; exact turn/version/digest and audit/publication prerequisites stay program-verified, never internal ledgers",
		"result_format": "report state.result_instruction remains fixed; audit execute format remains part of overridable body",
		"commit":        CommitInstruction, "success": SuccessLabel, "feedback": FeedbackLabel,
		"publication": map[string]any{"label": PublicationLabel, "slots": "execute only: provider/repository/base_url/remote/base; no provider query or authentication configuration"},
		"progress":    map[string]any{"text": ProgressInstruction, "continue": ContinueProgressInstruction, "when": "enabled implementation/audit execute only; reuse rule only if already supplied in this original session"},
		"result_file": map[string]any{"text": ResultFileInstruction, "continue": ContinueResultInstruction, "when": "enabled implementation/audit execute only; reuse rule only if already supplied in this original session"},
	}
}

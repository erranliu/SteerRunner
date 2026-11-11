package workflow

// Content definitions select existing execution capabilities. They have no
// acquisition fallback, publication edge or autonomous follow-up prompt.
func contentProfile(id string, version int) Profile {
	state := State{Kind: "execute", Contract: "final", ModelRole: "execution", Next: "closed", PolicySlots: []string{}}
	states := map[string]State{"execute": state, "closed": {Kind: "terminal", PolicySlots: []string{}}}
	if id == "conversation" {
		state.Next = "input"
		states["execute"] = state
		states["input"] = State{Kind: "input", Continue: "execute", PolicySlots: []string{}}
	}
	description := "Read-only content execution with original native final results."
	if ManagedConversation(id, version) {
		description = "Multi-turn local Codex conversation in one managed writable worktree."
	}
	return Profile{ID: id, Version: version, Initial: "execute", Description: description, SupportedAdapters: []string{"codex-app-server"}, PromptSteps: []string{}, States: states}
}

func Content(id string) bool { return id == "conversation" || id == "generate" }

func ManagedConversation(id string, version int) bool { return id == "conversation" && version == 2 }

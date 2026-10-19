package workflow

// Content definitions select existing execution capabilities. They have no
// acquisition fallback, publication edge or autonomous follow-up prompt.
func contentProfile(id string) Profile {
	state := State{Kind: "execute", Contract: "final", ModelRole: "execution", Next: "closed"}
	states := map[string]State{"execute": state, "closed": {Kind: "terminal"}}
	if id == "conversation" {
		state.Next = "input"
		states["execute"] = state
		states["input"] = State{Kind: "input", Continue: "execute"}
	}
	return Profile{ID: id, Version: 1, Initial: "execute", Description: "Read-only content execution with original native final results.", SupportedAdapters: []string{"codex-app-server"}, PromptSteps: []string{}, States: states}
}

func Content(id string) bool { return id == "conversation" || id == "generate" }

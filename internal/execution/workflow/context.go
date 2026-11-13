package workflow

import (
	_ "embed"
	"encoding/json"
)

//go:embed publication-draft.md
var draftPublicationInstruction string

func DraftPublicationInstruction() string { return draftPublicationInstruction }

func PublicationInstruction(state State, policy json.RawMessage, remote, base string) string {
	if state.Contract != "publication" || state.Kind != "execute" {
		return ""
	}
	var p struct {
		Provider   string `json:"provider"`
		Repository string `json:"repository"`
		Settings   struct {
			BaseURL string `json:"base_url"`
			Draft   bool   `json:"draft,omitempty"`
		} `json:"settings"`
	}
	_ = json.Unmarshal(policy, &p)
	target, _ := json.Marshal(struct {
		Provider   string `json:"provider"`
		Repository string `json:"repository"`
		BaseURL    string `json:"base_url,omitempty"`
		Remote     string `json:"remote"`
		Base       string `json:"base"`
		Draft      bool   `json:"draft,omitempty"`
	}{p.Provider, p.Repository, p.Settings.BaseURL, remote, base, p.Settings.Draft})
	if p.Settings.Draft {
		return PublicationLabel + string(target) + draftPublicationInstruction
	}
	return PublicationLabel + string(target)
}

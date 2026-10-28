package workflow

import "encoding/json"

func PublicationInstruction(state State, policy json.RawMessage, remote, base string) string {
	if state.Contract != "publication" || state.Kind != "execute" {
		return ""
	}
	var p struct {
		Provider   string `json:"provider"`
		Repository string `json:"repository"`
		Settings   struct {
			BaseURL string `json:"base_url"`
		} `json:"settings"`
	}
	_ = json.Unmarshal(policy, &p)
	target, _ := json.Marshal(struct {
		Provider   string `json:"provider"`
		Repository string `json:"repository"`
		BaseURL    string `json:"base_url,omitempty"`
		Remote     string `json:"remote"`
		Base       string `json:"base"`
	}{p.Provider, p.Repository, p.Settings.BaseURL, remote, base})
	return PublicationLabel + string(target)
}

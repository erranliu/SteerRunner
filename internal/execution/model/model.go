// Package model contains native model preferences without workflow semantics.
package model

type Settings struct {
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

// Bindings are constructed from frozen business configuration by the caller.
// Keys are opaque to a native transport; nil means a known but missing choice.
type Bindings map[string]*Settings

func (b Bindings) Clone() Bindings {
	if b == nil {
		return nil
	}
	out := make(Bindings, len(b))
	for key, value := range b {
		if value == nil {
			out[key] = nil
		} else {
			copied := *value
			out[key] = &copied
		}
	}
	return out
}

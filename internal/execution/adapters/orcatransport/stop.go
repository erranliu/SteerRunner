package orcatransport

type stopProgress struct {
	Version int    `json:"version"`
	Phase   string `json:"phase"`
}

// Only an identity-bound committed receipt proving no close was attempted can
// authorize the first close during reconciliation. A crash with no receipt or
// a lost close response remains unknown; neither permits repeating an effect.

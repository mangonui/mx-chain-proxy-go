package data

// DrwaDetails holds DRWA-specific client-facing materialized details.
type DrwaDetails struct {
	IsDrwa              bool     `json:"isDrwa,omitempty"`
	DenialCode          string   `json:"denialCode,omitempty"`
	DenialMessage       string   `json:"denialMessage,omitempty"`
	DenialTopics        []string `json:"denialTopics,omitempty"`
	HasComplianceSignal bool     `json:"hasComplianceSignal,omitempty"`
}

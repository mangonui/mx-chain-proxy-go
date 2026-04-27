package data

// MrvDetails holds MRV-specific client-facing materialized transaction details.
type MrvDetails struct {
	IsMrv                bool   `json:"isMrv,omitempty"`
	HasAnchoredProof     bool   `json:"hasAnchoredProof,omitempty"`
	SourceEvent          string `json:"sourceEvent,omitempty"`
	ReportID             string `json:"reportId,omitempty"`
	PublicTenantID       string `json:"publicTenantId,omitempty"`
	PublicFarmID         string `json:"publicFarmId,omitempty"`
	PublicSeasonID       string `json:"publicSeasonId,omitempty"`
	PublicProjectID      string `json:"publicProjectId,omitempty"`
	ReportHash           string `json:"reportHash,omitempty"`
	HashAlgo             string `json:"hashAlgo,omitempty"`
	Canonicalization     string `json:"canonicalization,omitempty"`
	MethodologyVersion   uint64 `json:"methodologyVersion,omitempty"`
	AnchoredAt           uint64 `json:"anchoredAt,omitempty"`
	EvidenceManifestHash string `json:"evidenceManifestHash,omitempty"`
}

package models

// NetBirdModePlanRequest is the request body for planning a NetBird mode change.
type NetBirdModePlanRequest struct {
	TargetMode      string `json:"targetMode"`
	AllowLocalhost  bool   `json:"allowLocalhost"`
	ModeBProjectIDs []uint `json:"modeBProjectIds,omitempty"`
}

// NetBirdModeConfigUpsertRequest is the request body for updating NetBird mode config.
type NetBirdModeConfigUpsertRequest struct {
	APIBaseURL      *string   `json:"apiBaseUrl,omitempty"`
	APIToken        *string   `json:"apiToken,omitempty"`
	HostPeerID      *string   `json:"hostPeerId,omitempty"`
	AdminPeerIDs    *[]string `json:"adminPeerIds,omitempty"`
	ModeBProjectIDs *[]uint   `json:"modeBProjectIds,omitempty"`
}

package model

type WikiProposalAgentView struct {
	Kind   string  `json:"kind"`
	Slug   string  `json:"slug"`
	Title  string  `json:"title"`
	Parent *string `json:"parent,omitempty"`
	Reason string  `json:"reason"`
}

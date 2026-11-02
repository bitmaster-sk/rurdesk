package model

import "encoding/json"

type StageExecutePayload struct {
	IdTask        int64            `json:"idTask"`
	Stage         string           `json:"stage"`
	AttemptNo     int              `json:"attemptNo"`
	PrBranch      string           `json:"prBranch"`
	Repo          *AgentRunGitRepo `json:"repo"`
	Model         string           `json:"model"`
	MaxTurns      int              `json:"maxTurns"`
	ContextBundle *ContextBundle   `json:"contextBundle"`
}

// List elements stay raw so one malformed entry is skipped instead of failing the whole stage.
type ContextBundle struct {
	Issue             *Issue                `json:"issue"`
	ReviewThread      []json.RawMessage     `json:"reviewThread"`
	ApprovedDesign    *Message              `json:"approvedDesign"`
	ApprovedImplPlan  *Message              `json:"approvedImplPlan"`
	RejectedOutput    *Message              `json:"rejectedOutput"`
	ApprovedMockupRef *string               `json:"approvedMockupRef"`
	Skills            []json.RawMessage     `json:"skills"`
	Wiki              *WikiPromptContext    `json:"wiki"`
	WikiProposals     []json.RawMessage     `json:"wikiProposals"`
	IsShortWorkflow   bool                  `json:"isShortWorkflow"`
	Release           *ReleasePromptContext `json:"release"`
}

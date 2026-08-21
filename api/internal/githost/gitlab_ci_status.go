package githost

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
)

// Only terminal states are enumerated; an unknown state is in flight, not a verdict.
func (h *GitLabHost) getCiStatus(ctx context.Context, idMr, sha string) string {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%s/pipelines", h.baseUrl, h.encodedPath, idMr)
	var pipelines []struct {
		Status string `json:"status"`
	}
	if err := getJSON(ctx, h.client, apiURL, h.setAuth, &pipelines); err != nil || len(pipelines) == 0 {
		return constants.CiStatusUnknown
	}
	switch pipelines[0].Status {
	case "success":
		return constants.CiStatusSuccess
	case "failed":
		return constants.CiStatusFailed
	case "canceled", "canceling":
		return constants.CiStatusCanceled
	case "skipped":
		return constants.CiStatusSkipped
	default:
		return constants.CiStatusPending
	}
}

// The merge request object has no `approved_by` key on any tier — only this endpoint.
func (h *GitLabHost) hasApproval(ctx context.Context, idMr string) bool {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%s/approvals", h.baseUrl, h.encodedPath, idMr)
	var payload struct {
		Approved   bool  `json:"approved"`
		ApprovedBy []any `json:"approved_by"`
	}
	if err := getJSON(ctx, h.client, apiURL, h.setAuth, &payload); err != nil {
		return false
	}
	return payload.Approved || len(payload.ApprovedBy) > 0
}

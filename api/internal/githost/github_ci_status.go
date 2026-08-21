package githost

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
)

const (
	githubCheckRunsPageSize = 100
	githubCheckRunsMaxPages = 10
)

// Actions reports through check runs; commit statuses cover external CI only.
func (h *GitHubHost) getCiStatus(ctx context.Context, idMr, sha string) string {
	jobs := h.fetchCheckRunJobs(ctx, sha)
	if len(jobs) == 0 {
		jobs = fetchCommitStatusJobs(ctx, h.client,
			fmt.Sprintf("%s/repos/%s/commits/%s/status", h.apiBase, h.repoPath, sha), h.setAuth)
	}
	return aggregateCiStatus(jobs)
}

// Paging is not optional: a failure can sit on the second page.
func (h *GitHubHost) fetchCheckRunJobs(ctx context.Context, sha string) []string {
	var jobs []string
	for page := 1; page <= githubCheckRunsMaxPages; page++ {
		url := fmt.Sprintf("%s/repos/%s/commits/%s/check-runs?filter=latest&per_page=%d&page=%d",
			h.apiBase, h.repoPath, sha, githubCheckRunsPageSize, page)
		var payload struct {
			TotalCount int `json:"total_count"`
			CheckRuns  []struct {
				Status     string `json:"status"`
				Conclusion string `json:"conclusion"`
			} `json:"check_runs"`
		}
		if err := getJSON(ctx, h.client, url, h.setAuth, &payload); err != nil {
			return jobs
		}
		for _, run := range payload.CheckRuns {
			if run.Status != "completed" {
				jobs = append(jobs, constants.CiStatusPending)
				continue
			}
			switch run.Conclusion {
			case "success", "neutral":
				jobs = append(jobs, constants.CiStatusSuccess)
			case "failure", "timed_out", "action_required", "startup_failure":
				jobs = append(jobs, constants.CiStatusFailed)
			case "cancelled":
				jobs = append(jobs, constants.CiStatusCanceled)
			case "skipped":
				jobs = append(jobs, constants.CiStatusSkipped)
			default:
				jobs = append(jobs, constants.CiStatusUnknown)
			}
		}
		if len(payload.CheckRuns) == 0 || len(jobs) >= payload.TotalCount {
			break
		}
	}
	return jobs
}

func (h *GitHubHost) hasApproval(ctx context.Context, idMr string) bool {
	return hasReviewApproval(ctx, h.client,
		fmt.Sprintf("%s/repos/%s/pulls/%s/reviews", h.apiBase, h.repoPath, idMr), h.setAuth)
}

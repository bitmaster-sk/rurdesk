package githost

import (
	"context"
	"fmt"
)

func (h *GiteaHost) getCiStatus(ctx context.Context, idMr, sha string) string {
	return aggregateCiStatus(fetchCommitStatusJobs(ctx, h.client,
		fmt.Sprintf("%s/api/v1/repos/%s/commits/%s/status", h.baseUrl, h.repoPath, sha), h.setAuth))
}

func (h *GiteaHost) hasApproval(ctx context.Context, idMr string) bool {
	return hasReviewApproval(ctx, h.client,
		fmt.Sprintf("%s/api/v1/repos/%s/pulls/%s/reviews", h.baseUrl, h.repoPath, idMr), h.setAuth)
}

package githost

import (
	"context"
	"net/http"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
)

// Order is load-bearing: a running job must not mask an already-failed one.
func aggregateCiStatus(jobs []string) string {
	var hasPending, hasSuccess, hasCanceled, hasSkipped bool
	for _, job := range jobs {
		switch job {
		case constants.CiStatusFailed:
			return constants.CiStatusFailed
		case constants.CiStatusPending:
			hasPending = true
		case constants.CiStatusSuccess:
			hasSuccess = true
		case constants.CiStatusCanceled:
			hasCanceled = true
		case constants.CiStatusSkipped:
			hasSkipped = true
		}
	}
	switch {
	case hasPending:
		return constants.CiStatusPending
	case hasCanceled:
		return constants.CiStatusCanceled
	case hasSuccess:
		return constants.CiStatusSuccess
	case hasSkipped:
		return constants.CiStatusSkipped
	default:
		return constants.CiStatusUnknown
	}
}

// Reads statuses[], not the top-level state, which is "pending" even with no CI.
func fetchCommitStatusJobs(ctx context.Context, client *http.Client, url string, setAuth func(*http.Request)) []string {
	var payload struct {
		Statuses []struct {
			State string `json:"state"`
		} `json:"statuses"`
	}
	if err := getJSON(ctx, client, url, setAuth, &payload); err != nil {
		return nil
	}
	jobs := make([]string, 0, len(payload.Statuses))
	for _, status := range payload.Statuses {
		switch status.State {
		case "success":
			jobs = append(jobs, constants.CiStatusSuccess)
		case "failure", "error":
			jobs = append(jobs, constants.CiStatusFailed)
		case "pending":
			jobs = append(jobs, constants.CiStatusPending)
		default:
			jobs = append(jobs, constants.CiStatusUnknown)
		}
	}
	return jobs
}

func hasReviewApproval(ctx context.Context, client *http.Client, url string, setAuth func(*http.Request)) bool {
	var reviews []struct {
		State string `json:"state"`
	}
	if err := getJSON(ctx, client, url, setAuth, &reviews); err != nil {
		return false
	}
	for _, review := range reviews {
		if review.State == "APPROVED" {
			return true
		}
	}
	return false
}

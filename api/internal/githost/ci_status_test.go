package githost

import (
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/stretchr/testify/assert"
)

func TestAggregateCiStatus(t *testing.T) {
	tests := []struct {
		name string
		jobs []string
		want string
	}{
		{name: "no jobs at all", jobs: nil, want: constants.CiStatusUnknown},
		{name: "all green", jobs: []string{constants.CiStatusSuccess, constants.CiStatusSuccess}, want: constants.CiStatusSuccess},
		{name: "one failure among green", jobs: []string{constants.CiStatusSuccess, constants.CiStatusFailed, constants.CiStatusSuccess}, want: constants.CiStatusFailed},
		{name: "one still running among green", jobs: []string{constants.CiStatusSuccess, constants.CiStatusPending}, want: constants.CiStatusPending},
		{name: "failure outranks running", jobs: []string{constants.CiStatusPending, constants.CiStatusFailed}, want: constants.CiStatusFailed},
		{name: "failure outranks canceled", jobs: []string{constants.CiStatusCanceled, constants.CiStatusFailed}, want: constants.CiStatusFailed},
		{name: "canceled outranks success", jobs: []string{constants.CiStatusSuccess, constants.CiStatusCanceled}, want: constants.CiStatusCanceled},
		{name: "running outranks canceled", jobs: []string{constants.CiStatusCanceled, constants.CiStatusPending}, want: constants.CiStatusPending},
		{name: "success outranks skipped", jobs: []string{constants.CiStatusSkipped, constants.CiStatusSuccess}, want: constants.CiStatusSuccess},
		{name: "everything skipped", jobs: []string{constants.CiStatusSkipped, constants.CiStatusSkipped}, want: constants.CiStatusSkipped},
		{name: "only unrecognised results", jobs: []string{constants.CiStatusUnknown, constants.CiStatusUnknown}, want: constants.CiStatusUnknown},
		{name: "unrecognised alongside green", jobs: []string{constants.CiStatusUnknown, constants.CiStatusSuccess}, want: constants.CiStatusSuccess},
		{name: "an already aggregated result stays itself", jobs: []string{constants.CiStatusCanceled}, want: constants.CiStatusCanceled},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, aggregateCiStatus(tc.jobs))
		})
	}
}

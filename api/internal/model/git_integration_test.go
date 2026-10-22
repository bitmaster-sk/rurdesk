package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGitIntegrationReq_GetNormalizedBaseBranch(t *testing.T) {
	tests := []struct {
		name       string
		baseBranch *string
		want       *string
	}{
		{name: "unset", baseBranch: nil, want: nil},
		{name: "blank means the repository default", baseBranch: ptr("   "), want: nil},
		{name: "trimmed", baseBranch: ptr("  develop "), want: ptr("develop")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, (&CreateGitIntegrationReq{BaseBranch: tc.baseBranch}).NormalizedBaseBranch())
			require.Equal(t, tc.want, (&UpdateGitIntegrationReq{BaseBranch: tc.baseBranch}).NormalizedBaseBranch())
		})
	}
}

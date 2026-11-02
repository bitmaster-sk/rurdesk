package service

import (
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

func releaseIssue(idPublic int64, idType *int64, isClosed bool) *model.ReleaseIssue {
	return &model.ReleaseIssue{IdIssuePublic: idPublic, IdIssueType: idType, IsClosed: isClosed}
}

func TestBuildReleaseDraftGroupsConsecutiveTypesAndSplitsOffOpenTasks(t *testing.T) {
	bug, feature := int64(1), int64(2)
	draft := buildReleaseDraft([]*model.ReleaseIssue{
		releaseIssue(10, &bug, true),
		releaseIssue(11, &bug, false),
		releaseIssue(12, &bug, true),
		releaseIssue(20, &feature, true),
		releaseIssue(30, nil, true),
		releaseIssue(31, nil, true),
	})

	require.Len(t, draft.Sections, 3)
	require.Len(t, draft.Sections[0].Issues, 2)
	require.Equal(t, int64(12), draft.Sections[0].Issues[1].IdIssuePublic)
	require.Equal(t, feature, *draft.Sections[1].IdIssueType)
	require.Nil(t, draft.Sections[2].IdIssueType)
	require.Len(t, draft.Sections[2].Issues, 2)
	require.Len(t, draft.OpenIssues, 1)
	require.Equal(t, 5, releasedIssueCount(&model.Release{OpenIssues: constants.ReleaseOpenIssuesMove}, draft))
	require.Equal(t, 6, releasedIssueCount(&model.Release{OpenIssues: constants.ReleaseOpenIssuesKeep}, draft))
}

func TestBuildReleaseDraftOfNothingIsEmptyNotNil(t *testing.T) {
	draft := buildReleaseDraft(nil)
	require.NotNil(t, draft.Sections)
	require.NotNil(t, draft.OpenIssues)
	require.Zero(t, releasedIssueCount(&model.Release{OpenIssues: constants.ReleaseOpenIssuesKeep}, draft))
}

package constants

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSkillStagesCoverEveryRunKindButPickup(t *testing.T) {
	require.Equal(t, []string{StageBrainstorming, StageDesign, StageImplementationPlan, StageImplementation, StageReleaseNotes}, SkillStages)
	require.False(t, IsSkillStage(StagePickup))
	require.True(t, IsSkillStage(StageReleaseNotes))
}

func TestStagesOfKind(t *testing.T) {
	release := StagesOfKind(RunKindReleaseNotes)
	require.Equal(t, []StageDef{{Name: StagePickup}, {Name: StageReleaseNotes}}, release)
	require.Equal(t, StageDefinitions, StagesOfKind(RunKindIssue))
	require.Equal(t, StageDefinitions, StagesOfKind("unknown"), "an unknown kind falls back to the development stages")
}

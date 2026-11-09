import { AgentStageDefaultsGroup } from '../entity/agent-stage-defaults-group.entity';
import { AgentStage } from '../model/agent-stage.enum';
import { RUN_KIND_LABELS, RunKindName } from './run-kind.enum';
import { SKILL_STAGES } from './skill-stages.enum';

export const AGENT_STAGE_DEFAULTS_GROUPS: AgentStageDefaultsGroup[] = [
    { label: RUN_KIND_LABELS[RunKindName.Issue], stages: SKILL_STAGES },
    { label: RUN_KIND_LABELS[RunKindName.ReleaseNotes], stages: [AgentStage.ReleaseNotes] }
];

export const AGENT_DEFAULT_STAGES: AgentStage[] = AGENT_STAGE_DEFAULTS_GROUPS.flatMap(
    group => group.stages
);

import { AgentEffort } from '../model/agent-effort.enum';
import { AgentStage } from '../model/agent-stage.enum';
import { AgentEffortInherited } from './agent-effort.entity';

export enum AgentStagePlanEditorLock {
    Skipped = 'skipped'
}

export interface AgentStagePlanEditorRow {
    stage: AgentStage;
    idsSkill: number[];
    model: string | null;
    maxTurns: number | null;
    defaultMaxTurns: number | null;
    effort: AgentEffort | null;
    inheritedEffort: AgentEffortInherited | null;
    isThinkingSupported: boolean;
    lock: AgentStagePlanEditorLock | null;
}

export interface AgentStagePlanSkillToggle {
    stage: AgentStage;
    idSkill: number;
}

export interface AgentStagePlanModelPick {
    stage: AgentStage;
    model: string | null;
}

export interface AgentStagePlanEffortPick {
    stage: AgentStage;
    effort: AgentEffort | null;
}

export interface AgentStagePlanMaxTurnsChange {
    stage: AgentStage;
    maxTurns: number | null;
}

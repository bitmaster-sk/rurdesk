import { AgentStage } from '../model/agent-stage.enum';

export enum AgentStagePlanEditorLock {
    Skipped = 'skipped',
    Started = 'started'
}

export interface AgentStagePlanEditorRow {
    stage: AgentStage;
    idsSkill: number[];
    model: string | null;
    maxTurns: number | null;
    defaultMaxTurns: number | null;
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

export interface AgentStagePlanMaxTurnsChange {
    stage: AgentStage;
    maxTurns: number | null;
}

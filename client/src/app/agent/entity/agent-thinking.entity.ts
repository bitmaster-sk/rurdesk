import { AgentThinkingKind } from '../constants/agent-thinking-kind.enum';
import { AgentToolKind } from '../constants/agent-tool-kind.enum';
import { AgentStageProgress } from '../model/agent-run.model';

export interface AgentThinkingLine {
    kind: AgentThinkingKind;
    label: string;
    detail: string;
    toolKind: AgentToolKind;
}

export interface AgentThinkingRow {
    stage: AgentStageProgress;
    isLive: boolean;
}

import { AgentEffortInherited, AgentEffortOrigin } from '../entity/agent-effort.entity';
import { AgentEffort, AgentEffortSource } from '../model/agent-effort.enum';
import { AgentDefaultsByStage, AgentEffortDefault } from '../model/agent-stage-options.model';
import { AgentStage } from '../model/agent-stage.enum';

export abstract class AgentEffortConverter {
    public static toInherited(
        stage: AgentStage,
        runEffort: AgentEffort | null,
        defaultsByStage: AgentDefaultsByStage | null,
        agentName: string | null
    ): AgentEffortInherited | null {
        if (runEffort) {
            return { effort: runEffort, origin: AgentEffortOrigin.Run, agentName: null };
        }
        const stageDefault = defaultsByStage?.[stage];
        return stageDefault
            ? AgentEffortConverter.toAgentInherited(stageDefault.effort, agentName)
            : null;
    }

    public static toAgentInherited(
        effortDefault: AgentEffortDefault,
        agentName: string | null
    ): AgentEffortInherited {
        return {
            effort: effortDefault.effort,
            origin: AgentEffortConverter.toOrigin(effortDefault.source),
            agentName
        };
    }

    public static toHintKey(inherited: AgentEffortInherited): string {
        if (!inherited.effort) {
            return 'AGENT_EFFORT.DEFAULT_FROM_MODEL';
        }
        switch (inherited.origin) {
            case AgentEffortOrigin.Run:
                return 'AGENT_EFFORT.DEFAULT_FROM_RUN';
            case AgentEffortOrigin.Agent:
                return 'AGENT_EFFORT.DEFAULT_FROM_AGENT';
            default:
                return 'AGENT_EFFORT.DEFAULT_FROM_SETTINGS';
        }
    }

    public static isThinkingSupported(
        model: string | null,
        modelsWithoutThinking: readonly string[]
    ): boolean {
        return model === null || !modelsWithoutThinking.includes(model);
    }

    private static toOrigin(source: AgentEffortSource): AgentEffortOrigin {
        switch (source) {
            case AgentEffortSource.Agent:
                return AgentEffortOrigin.Agent;
            case AgentEffortSource.Settings:
                return AgentEffortOrigin.Settings;
            default:
                return AgentEffortOrigin.Model;
        }
    }
}

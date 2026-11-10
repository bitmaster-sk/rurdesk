import { describe, expect, it } from 'vitest';
import { AgentEffortConverter } from './agent-effort.converter';
import { AgentEffortOrigin } from '../entity/agent-effort.entity';
import { AgentEffort, AgentEffortSource } from '../model/agent-effort.enum';
import { AgentDefaultsByStage } from '../model/agent-stage-options.model';
import { AgentStage } from '../model/agent-stage.enum';

const defaults: AgentDefaultsByStage = {
    [AgentStage.Brainstorming]: {
        maxTurns: 250,
        effort: { effort: AgentEffort.High, source: AgentEffortSource.Agent }
    },
    [AgentStage.Design]: {
        maxTurns: 250,
        effort: { effort: AgentEffort.Medium, source: AgentEffortSource.Settings }
    },
    [AgentStage.Implementation]: {
        maxTurns: 500,
        effort: { effort: null, source: AgentEffortSource.Model }
    }
};

describe('AgentEffortConverter.toInherited', () => {
    it('takes the run effort before any agent default', () => {
        expect(
            AgentEffortConverter.toInherited(AgentStage.Design, AgentEffort.Low, defaults, 'Kimi')
        ).toEqual({ effort: AgentEffort.Low, origin: AgentEffortOrigin.Run, agentName: null });
    });

    it('takes the default of that stage', () => {
        expect(
            AgentEffortConverter.toInherited(AgentStage.Brainstorming, null, defaults, 'Kimi')
        ).toEqual({ effort: AgentEffort.High, origin: AgentEffortOrigin.Agent, agentName: 'Kimi' });
        expect(
            AgentEffortConverter.toInherited(AgentStage.Implementation, null, defaults, 'Kimi')
        ).toEqual({ effort: null, origin: AgentEffortOrigin.Model, agentName: 'Kimi' });
    });

    it('names the Rurdesk settings as the source of a settings default', () => {
        expect(
            AgentEffortConverter.toInherited(AgentStage.Design, null, defaults, 'Kimi')?.origin
        ).toBe(AgentEffortOrigin.Settings);
    });

    it('knows nothing before the defaults are loaded', () => {
        expect(AgentEffortConverter.toInherited(AgentStage.Design, null, null, 'Kimi')).toBeNull();
        expect(
            AgentEffortConverter.toInherited(AgentStage.ReleaseNotes, null, defaults, 'Kimi')
        ).toBeNull();
    });
});

describe('AgentEffortConverter.isThinkingSupported', () => {
    it('is false for a model on the list without thinking', () => {
        expect(AgentEffortConverter.isThinkingSupported('openai/gpt-4o', ['openai/gpt-4o'])).toBe(
            false
        );
    });

    it('is true for an unknown model and when no model is known yet', () => {
        expect(AgentEffortConverter.isThinkingSupported('ollama/qwen3:8b', ['openai/gpt-4o'])).toBe(
            true
        );
        expect(AgentEffortConverter.isThinkingSupported(null, ['openai/gpt-4o'])).toBe(true);
    });
});

describe('AgentEffortConverter.toHintKey', () => {
    it('names where an inherited effort comes from', () => {
        const hint = (origin: AgentEffortOrigin) =>
            AgentEffortConverter.toHintKey({ effort: AgentEffort.High, origin, agentName: 'Kimi' });

        expect(hint(AgentEffortOrigin.Run)).toBe('AGENT_EFFORT.DEFAULT_FROM_RUN');
        expect(hint(AgentEffortOrigin.Agent)).toBe('AGENT_EFFORT.DEFAULT_FROM_AGENT');
        expect(hint(AgentEffortOrigin.Settings)).toBe('AGENT_EFFORT.DEFAULT_FROM_SETTINGS');
    });

    it('leaves the effort to the model when nothing sets one', () => {
        expect(
            AgentEffortConverter.toHintKey({
                effort: null,
                origin: AgentEffortOrigin.Model,
                agentName: 'Kimi'
            })
        ).toBe('AGENT_EFFORT.DEFAULT_FROM_MODEL');
    });
});

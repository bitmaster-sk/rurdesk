import { Injector, runInInjectionContext } from '@angular/core';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { describe, expect, it } from 'vitest';
import { AgentEffortInherited, AgentEffortOrigin } from '../entity/agent-effort.entity';
import { AgentEffort } from '../model/agent-effort.enum';
import { AgentStage } from '../model/agent-stage.enum';
import { AgentEffortHintService } from './agent-effort-hint.service';

const TRANSLATIONS: Record<string, string> = {
    'AGENT_EFFORT.LEVEL.HIGH': 'High',
    'AGENT_EFFORT.DEFAULT_FROM_AGENT': 'Default: {{effort}} (from agent {{name}})',
    'AGENT_EFFORT.DEFAULT_FROM_MODEL': 'Default: from the model',
    'AGENT.STAGE.BRAINSTORMING': 'Brainstorming',
    'AGENT.STAGE.DESIGN': 'Design',
    'AGENT.STAGE.IMPLEMENTATION': 'Implementation'
};
const FROM_AGENT: AgentEffortInherited = {
    effort: AgentEffort.High,
    origin: AgentEffortOrigin.Agent,
    agentName: 'Kimi'
};
const FROM_MODEL: AgentEffortInherited = {
    effort: null,
    origin: AgentEffortOrigin.Model,
    agentName: 'Kimi'
};

function make(): AgentEffortHintService {
    const i18n = {
        instant: (key: string, params: Record<string, string | null> = {}) =>
            (TRANSLATIONS[key] ?? key).replace(
                /{{(\w+)}}/g,
                (_, name: string) => params[name] ?? ''
            )
    } as unknown as I18nService;
    const injector = Injector.create({ providers: [{ provide: I18nService, useValue: i18n }] });
    return runInInjectionContext(injector, () => new AgentEffortHintService());
}

describe('AgentEffortHintService', () => {
    it('names the inherited effort and where it comes from', () => {
        expect(make().describe(FROM_AGENT)).toBe('Default: High (from agent Kimi)');
    });

    it('gives one line when every stage shares the default', () => {
        expect(
            make().describeStages([
                { stage: AgentStage.Brainstorming, inherited: FROM_MODEL },
                { stage: AgentStage.Implementation, inherited: FROM_MODEL }
            ])
        ).toBe('Default: from the model');
    });

    it('gives a line per default and names the stages that share it', () => {
        expect(
            make().describeStages([
                { stage: AgentStage.Brainstorming, inherited: FROM_AGENT },
                { stage: AgentStage.Design, inherited: FROM_MODEL },
                { stage: AgentStage.Implementation, inherited: FROM_AGENT }
            ])
        ).toBe(
            'Brainstorming, Implementation · Default: High (from agent Kimi)\nDesign · Default: from the model'
        );
    });
});

import { Injectable, inject } from '@angular/core';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { AGENT_EFFORT_LABELS } from '../constants/agent-effort.constants';
import { AgentEffortConverter } from '../converter/agent-effort.converter';
import { AgentEffortInherited, AgentStageEffortInherited } from '../entity/agent-effort.entity';
import { STAGE_LABELS } from '../model/agent-stage.enum';

@Injectable({ providedIn: 'root' })
export class AgentEffortHintService {
    private readonly i18n = inject(I18nService);

    public describe(inherited: AgentEffortInherited): string {
        return this.i18n.instant(AgentEffortConverter.toHintKey(inherited), {
            effort: inherited.effort
                ? this.i18n.instant(AGENT_EFFORT_LABELS[inherited.effort])
                : '',
            name: inherited.agentName
        });
    }

    public describeStages(stages: AgentStageEffortInherited[]): string {
        const stagesByHint = new Map<string, string[]>();
        stages.forEach(({ stage, inherited }) => {
            const hint = this.describe(inherited);
            stagesByHint.set(hint, [
                ...(stagesByHint.get(hint) ?? []),
                this.i18n.instant(STAGE_LABELS[stage])
            ]);
        });
        if (stagesByHint.size === 1) {
            return [...stagesByHint.keys()][0];
        }
        return [...stagesByHint]
            .map(([hint, stageLabels]) => `${stageLabels.join(', ')} · ${hint}`)
            .join('\n');
    }
}

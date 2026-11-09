import { ChangeDetectionStrategy, Component, inject, input } from '@angular/core';
import { ControlContainer, FormGroupDirective } from '@angular/forms';
import { AGENT_EFFORT_LABELS, AGENT_EFFORTS } from 'src/app/agent/constants/agent-effort.constants';
import { AGENT_STAGE_DEFAULTS_GROUPS } from 'src/app/agent/constants/agent-stage-defaults.constants';
import { AgentStage, STAGE_LABELS } from 'src/app/agent/model/agent-stage.enum';
import { AgentStageOptionsByStage } from 'src/app/agent/model/agent-stage-options.model';
import { I18nService } from 'src/app/shared/i18n/i18n.service';

@Component({
    selector: 'app-agent-run-defaults-fields',
    templateUrl: './agent-run-defaults-fields.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush,
    viewProviders: [{ provide: ControlContainer, useExisting: FormGroupDirective }]
})
export class AgentRunDefaultsFieldsComponent {
    public readonly idPrefix = input.required<string>();
    public readonly groupName = input('stageDefaults');
    public readonly inherited = input<AgentStageOptionsByStage | null>(null);

    private readonly i18n = inject(I18nService);

    protected readonly efforts = AGENT_EFFORTS;
    protected readonly groups = AGENT_STAGE_DEFAULTS_GROUPS;
    protected readonly stageLabels = STAGE_LABELS;

    protected turnsPlaceholder(stage: AgentStage): string {
        const maxTurns = this.inherited()?.[stage]?.maxTurns;
        return maxTurns ? this.fromSettings(String(maxTurns)) : '';
    }

    protected effortPlaceholder(stage: AgentStage): string {
        const effort = this.inherited()?.[stage]?.effort;
        return effort
            ? this.fromSettings(this.i18n.instant(AGENT_EFFORT_LABELS[effort]))
            : this.i18n.instant('SETTINGS.AGENT_EFFORT_MODEL_DEFAULT');
    }

    private fromSettings(value: string): string {
        return this.i18n.instant('AGENT_API_KEY.FROM_SETTINGS', { value });
    }
}

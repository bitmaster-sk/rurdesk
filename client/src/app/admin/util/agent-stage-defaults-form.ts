import { FormControl, FormGroup, FormRecord, ValidatorFn, Validators } from '@angular/forms';
import { AGENT_DEFAULT_STAGES } from 'src/app/agent/constants/agent-stage-defaults.constants';
import { AgentEffort } from 'src/app/agent/model/agent-effort.enum';
import { AgentStage } from 'src/app/agent/model/agent-stage.enum';
import { AgentStageOptionsByStage } from 'src/app/agent/model/agent-stage-options.model';
import { AgentStageOptionsChange } from 'src/app/core/settings/model/app-settings.model';
import { MAX_TURNS_VALIDATORS } from '../constants/max-agent-turns';
import {
    AgentStageDefaultForm,
    AgentStageDefaultsForm
} from '../entity/agent-stage-defaults-form.entity';

export abstract class AgentStageDefaultsFormBuilder {
    public static build(isTurnLimitRequired: boolean): AgentStageDefaultsForm {
        const turnValidators: ValidatorFn[] = isTurnLimitRequired
            ? [Validators.required, ...MAX_TURNS_VALIDATORS]
            : MAX_TURNS_VALIDATORS;
        const form = new FormRecord<AgentStageDefaultForm>({});
        AGENT_DEFAULT_STAGES.forEach(stage =>
            form.addControl(
                stage,
                new FormGroup({
                    maxTurns: new FormControl<number | null>(null, turnValidators),
                    effort: new FormControl<AgentEffort | null>(null, { updateOn: 'change' })
                })
            )
        );
        return form;
    }

    public static toOptions(form: AgentStageDefaultsForm): AgentStageOptionsByStage {
        const defaults: AgentStageOptionsByStage = {};
        AGENT_DEFAULT_STAGES.forEach(stage => {
            const value = form.controls[stage].getRawValue();
            const maxTurns = value.maxTurns || null;
            const effort = value.effort || null;
            if (maxTurns !== null || effort !== null) {
                defaults[stage] = { maxTurns, effort };
            }
        });
        return defaults;
    }

    public static toSettingsChange(
        form: AgentStageDefaultsForm
    ): Partial<Record<AgentStage, AgentStageOptionsChange>> {
        const defaults: Partial<Record<AgentStage, AgentStageOptionsChange>> = {};
        AGENT_DEFAULT_STAGES.forEach(stage => {
            const value = form.controls[stage].getRawValue();
            defaults[stage] = { maxTurns: value.maxTurns ?? undefined, effort: value.effort ?? '' };
        });
        return defaults;
    }

    public static patch(
        form: AgentStageDefaultsForm,
        defaults: AgentStageOptionsByStage,
        options: { emitEvent?: boolean } = {}
    ): void {
        AGENT_DEFAULT_STAGES.forEach(stage =>
            form.controls[stage].setValue(
                {
                    maxTurns: defaults[stage]?.maxTurns ?? null,
                    effort: defaults[stage]?.effort ?? null
                },
                options
            )
        );
    }

    public static isEqual(
        left: AgentStageOptionsByStage,
        right: AgentStageOptionsByStage
    ): boolean {
        return AGENT_DEFAULT_STAGES.every(
            stage =>
                (left[stage]?.maxTurns ?? null) === (right[stage]?.maxTurns ?? null) &&
                (left[stage]?.effort ?? null) === (right[stage]?.effort ?? null)
        );
    }
}

import { ChangeDetectionStrategy, Component, DestroyRef, inject } from '@angular/core';
import { FormBuilder, Validators } from '@angular/forms';
import omit from 'lodash-es/omit';
import { SettingsApi } from 'src/app/core/settings/settings.api.service';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { AgentStageDefaultsFormBuilder } from '../../../util/agent-stage-defaults-form';
import { SettingsAutoSave } from '../../../util/settings-auto-save';

@Component({
    selector: 'app-admin-settings-agents',
    templateUrl: './admin-settings-agents.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AdminSettingsAgentsComponent {
    private readonly fb = inject(FormBuilder);

    protected readonly form = this.fb.nonNullable.group(
        {
            userApiKeyLimit: [10, [Validators.required, Validators.min(1), Validators.max(100)]],
            isAgentThinkingPersisted: this.fb.nonNullable.control(true, { updateOn: 'change' }),
            agentThinkingMaxKb: [
                1024,
                [Validators.required, Validators.min(64), Validators.max(10240)]
            ],
            agentStageDefaults: AgentStageDefaultsFormBuilder.build(true),
            wikiAlwaysTokenLimit: [
                12000,
                [Validators.required, Validators.min(0), Validators.max(60000)]
            ]
        },
        { updateOn: 'blur' }
    );

    private readonly autoSave = new SettingsAutoSave(
        this.form,
        {
            patch: settings => {
                this.form.patchValue(omit(settings, 'agentStageDefaults'), { emitEvent: false });
                AgentStageDefaultsFormBuilder.patch(
                    this.form.controls.agentStageDefaults,
                    settings.agentStageDefaults,
                    { emitEvent: false }
                );
            },
            toRequest: () => ({
                ...this.form.getRawValue(),
                agentStageDefaults: AgentStageDefaultsFormBuilder.toSettingsChange(
                    this.form.controls.agentStageDefaults
                )
            })
        },
        inject(SettingsApi),
        inject(SettingsStore),
        inject(DestroyRef)
    );

    protected readonly saveStatus = this.autoSave.status;

    protected onSave(): void {
        this.autoSave.save();
    }
}

import { ChangeDetectionStrategy, Component, DestroyRef, inject } from '@angular/core';
import { FormBuilder, Validators } from '@angular/forms';
import { SettingsApi } from 'src/app/core/settings/settings.api.service';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { SettingsAutoSave } from '../../../util/settings-auto-save';

@Component({
    selector: 'app-admin-settings-general',
    templateUrl: './admin-settings-general.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AdminSettingsGeneralComponent {
    private readonly fb = inject(FormBuilder);

    protected readonly form = this.fb.nonNullable.group(
        {
            tablePageSize: [50, [Validators.required, Validators.min(1), Validators.max(200)]],
            kanbanPageSize: [20, [Validators.required, Validators.min(1), Validators.max(200)]],
            ganttBacklogPageSize: [
                30,
                [Validators.required, Validators.min(1), Validators.max(200)]
            ],
            sprintVelocityLimit: [10, [Validators.required, Validators.min(1), Validators.max(50)]],
            attachmentMaxSizeMb: [
                25,
                [Validators.required, Validators.min(1), Validators.max(100)]
            ],
            plantUmlServerUrl: ['', [Validators.maxLength(500)]]
        },
        { updateOn: 'blur' }
    );

    private readonly autoSave = new SettingsAutoSave(
        this.form,
        {
            patch: settings => this.form.patchValue(settings, { emitEvent: false }),
            toRequest: () => this.form.getRawValue()
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

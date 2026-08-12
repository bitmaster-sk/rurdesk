import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormBuilder, Validators } from '@angular/forms';
import { filter } from 'rxjs/operators';
import { UiSaveState } from '../../../ui/components/save-status/save-status-chip.component';
import { AdminApi } from '../../api/admin.api.service';
import { VersionApi } from '../../api/version.api.service';
import { BuildInfo } from '../../model/build-info.model';

@Component({
    selector: 'app-admin-settings',
    templateUrl: './admin-settings.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AdminSettingsComponent {
    private readonly adminApi = inject(AdminApi);
    private readonly versionApi = inject(VersionApi);
    private readonly fb = inject(FormBuilder);

    protected readonly saveStatus = signal<UiSaveState>(UiSaveState.Idle);

    private readonly buildInfo = signal<BuildInfo | null>(null);

    /** e.g. "1.0.0 (abc1234)". An unstamped local build has no commit to show. */
    protected readonly versionLabel = computed(() => {
        const info = this.buildInfo();
        if (!info) {
            return '';
        }
        if (info.commit === 'unknown') {
            return info.version;
        }
        return `${info.version} (${info.commit.slice(0, 7)})`;
    });

    // All settings save together as one object → form-level
    // auto-save on blur, one panel-level chip (like the issue detail header).
    protected readonly form = this.fb.nonNullable.group(
        {
            tablePageSize: [50, [Validators.required, Validators.min(1), Validators.max(200)]],
            kanbanPageSize: [20, [Validators.required, Validators.min(1), Validators.max(200)]],
            ganttBacklogPageSize: [
                30,
                [Validators.required, Validators.min(1), Validators.max(200)]
            ],
            sprintVelocityLimit: [10, [Validators.required, Validators.min(1), Validators.max(50)]]
        },
        { updateOn: 'blur' }
    );

    /** Snapshot of the last persisted values so a blur without a real change
     *  doesn't fire a redundant save (and flash the chip). */
    private lastSaved = '';

    constructor() {
        this.versionApi
            .getVersion$()
            .pipe(takeUntilDestroyed())
            .subscribe(info => this.buildInfo.set(info));

        this.adminApi.getSettings$().subscribe(settings => {
            this.form.patchValue(settings, { emitEvent: false });
            this.lastSaved = JSON.stringify(this.form.getRawValue());
        });

        this.form.valueChanges
            .pipe(
                filter(
                    () =>
                        this.form.valid &&
                        JSON.stringify(this.form.getRawValue()) !== this.lastSaved
                ),
                takeUntilDestroyed()
            )
            .subscribe(() => this.onSave());
    }

    protected onSave(): void {
        if (this.form.invalid) {
            return;
        }
        this.saveStatus.set(UiSaveState.Saving);
        this.adminApi.updateSettings$(this.form.getRawValue()).subscribe({
            next: settings => {
                this.form.patchValue(settings, { emitEvent: false });
                this.lastSaved = JSON.stringify(this.form.getRawValue());
                this.saveStatus.set(UiSaveState.Saved);
            },
            error: () => this.saveStatus.set(UiSaveState.Error)
        });
    }
}

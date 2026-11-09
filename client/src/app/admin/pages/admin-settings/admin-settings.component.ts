import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormBuilder, Validators } from '@angular/forms';
import { filter } from 'rxjs/operators';
import { UiSaveState } from '../../../ui/components/save-status/save-status-chip.component';
import { SettingsApi } from 'src/app/core/settings/settings.api.service';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { VersionApi } from '../../api/version.api.service';
import { BuildInfo } from '../../model/build-info.model';
import omit from 'lodash-es/omit';
import { AppSettings } from 'src/app/core/settings/model/app-settings.model';
import { AgentStageDefaultsFormBuilder } from '../../util/agent-stage-defaults-form';

@Component({
    selector: 'app-admin-settings',
    templateUrl: './admin-settings.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AdminSettingsComponent {
    private readonly settingsApi = inject(SettingsApi);
    private readonly settingsStore = inject(SettingsStore);
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
            sprintVelocityLimit: [10, [Validators.required, Validators.min(1), Validators.max(50)]],
            userApiKeyLimit: [10, [Validators.required, Validators.min(1), Validators.max(100)]],
            isAgentThinkingPersisted: this.fb.nonNullable.control(true, { updateOn: 'change' }),
            agentThinkingMaxKb: [
                1024,
                [Validators.required, Validators.min(64), Validators.max(10240)]
            ],
            attachmentMaxSizeMb: [
                25,
                [Validators.required, Validators.min(1), Validators.max(100)]
            ],
            agentStageDefaults: AgentStageDefaultsFormBuilder.build(true),
            wikiAlwaysTokenLimit: [
                12000,
                [Validators.required, Validators.min(0), Validators.max(60000)]
            ],
            plantUmlServerUrl: ['', [Validators.maxLength(500)]]
        },
        { updateOn: 'blur' }
    );

    /** Snapshot of the last persisted values so a blur without a real change
     *  doesn't fire a redundant save (and flash the chip). */
    private lastSaved = '';

    public constructor() {
        this.versionApi
            .load$()
            .pipe(takeUntilDestroyed())
            .subscribe(info => this.buildInfo.set(info));

        this.settingsApi.load$().subscribe(settings => {
            this.patchForm(settings);
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

    private patchForm(settings: AppSettings): void {
        this.form.patchValue(omit(settings, 'agentStageDefaults'), { emitEvent: false });
        AgentStageDefaultsFormBuilder.patch(
            this.form.controls.agentStageDefaults,
            settings.agentStageDefaults,
            { emitEvent: false }
        );
    }

    protected onSave(): void {
        if (this.form.invalid) {
            return;
        }
        this.saveStatus.set(UiSaveState.Saving);
        const value = this.form.getRawValue();
        this.settingsApi
            .update$({
                ...value,
                agentStageDefaults: AgentStageDefaultsFormBuilder.toSettingsChange(
                    this.form.controls.agentStageDefaults
                )
            })
            .subscribe({
                next: settings => {
                    this.settingsStore.set(settings);
                    this.patchForm(settings);
                    this.lastSaved = JSON.stringify(this.form.getRawValue());
                    this.saveStatus.set(UiSaveState.Saved);
                },
                error: () => this.saveStatus.set(UiSaveState.Error)
            });
    }
}

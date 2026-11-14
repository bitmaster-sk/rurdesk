import { DestroyRef, WritableSignal, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormGroup } from '@angular/forms';
import { filter } from 'rxjs/operators';
import { SettingsApi } from 'src/app/core/settings/settings.api.service';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';
import { SettingsSection } from '../entity/settings-section.entity';

export class SettingsAutoSave {
    public readonly status: WritableSignal<UiSaveState> = signal(UiSaveState.Idle);

    private readonly form: FormGroup;
    private readonly section: SettingsSection;
    private readonly api: SettingsApi;
    private readonly store: SettingsStore;
    private lastSaved = '';

    public constructor(
        form: FormGroup,
        section: SettingsSection,
        api: SettingsApi,
        store: SettingsStore,
        destroyRef: DestroyRef
    ) {
        this.form = form;
        this.section = section;
        this.api = api;
        this.store = store;

        api.load$()
            .pipe(takeUntilDestroyed(destroyRef))
            .subscribe(settings => {
                section.patch(settings);
                this.lastSaved = this.snapshot();
            });

        form.valueChanges
            .pipe(
                filter(() => form.valid && this.snapshot() !== this.lastSaved),
                takeUntilDestroyed(destroyRef)
            )
            .subscribe(() => this.save());
    }

    public save(): void {
        if (this.form.invalid) {
            return;
        }
        this.status.set(UiSaveState.Saving);
        this.api.update$(this.section.toRequest()).subscribe({
            next: settings => {
                this.store.set(settings);
                this.section.patch(settings);
                this.lastSaved = this.snapshot();
                this.status.set(UiSaveState.Saved);
            },
            error: () => this.status.set(UiSaveState.Error)
        });
    }

    private snapshot(): string {
        return JSON.stringify(this.form.getRawValue());
    }
}

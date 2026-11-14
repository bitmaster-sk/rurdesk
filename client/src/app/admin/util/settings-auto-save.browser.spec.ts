import { Fixtures } from 'src/testing/fixtures';
import { DestroyRef } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormBuilder } from '@angular/forms';
import { of, throwError } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { SettingsApi } from 'src/app/core/settings/settings.api.service';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { AppSettings } from 'src/app/core/settings/model/app-settings.model';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';
import { SettingsAutoSave } from './settings-auto-save';

describe('SettingsAutoSave', () => {
    const loaded: AppSettings = {
        tablePageSize: 50,
        kanbanPageSize: 20,
        ganttBacklogPageSize: 30,
        sprintVelocityLimit: 10,
        userApiKeyLimit: 10,
        isAgentThinkingPersisted: true,
        agentThinkingMaxKb: 1024,
        attachmentMaxSizeMb: 25,
        agentStageDefaults: Fixtures.appAgentStageDefaults(),
        wikiAlwaysTokenLimit: 12000,
        plantUmlServerUrl: ''
    };

    function setup(update$ = vi.fn().mockReturnValue(of({ ...loaded, tablePageSize: 60 }))) {
        const store = { set: vi.fn() };
        TestBed.configureTestingModule({
            providers: [
                { provide: SettingsApi, useValue: { load$: () => of(loaded), update$ } },
                { provide: SettingsStore, useValue: store }
            ]
        });
        const form = new FormBuilder().nonNullable.group({ tablePageSize: 0 });
        const autoSave = TestBed.runInInjectionContext(
            () =>
                new SettingsAutoSave(
                    form,
                    {
                        patch: settings => form.patchValue(settings, { emitEvent: false }),
                        toRequest: () => form.getRawValue()
                    },
                    TestBed.inject(SettingsApi),
                    TestBed.inject(SettingsStore),
                    TestBed.inject(DestroyRef)
                )
        );
        return { form, autoSave, update$, store };
    }

    it('fills the form from the loaded settings without saving', () => {
        const { form, update$ } = setup();

        expect(form.controls.tablePageSize.value).toBe(50);
        expect(update$).not.toHaveBeenCalled();
    });

    it('saves only the fields of its own section and publishes the result', () => {
        const { form, update$, store, autoSave } = setup();

        form.controls.tablePageSize.setValue(60);

        expect(update$).toHaveBeenCalledWith({ tablePageSize: 60 });
        expect(store.set).toHaveBeenCalledWith({ ...loaded, tablePageSize: 60 });
        expect(autoSave.status()).toBe(UiSaveState.Saved);
    });

    it('does not save a value equal to the last saved one', () => {
        const { form, update$ } = setup();

        form.controls.tablePageSize.setValue(50);

        expect(update$).not.toHaveBeenCalled();
    });

    it('shows an error when the save fails', () => {
        const { form, autoSave } = setup(vi.fn().mockReturnValue(throwError(() => new Error())));

        form.controls.tablePageSize.setValue(70);

        expect(autoSave.status()).toBe(UiSaveState.Error);
    });
});

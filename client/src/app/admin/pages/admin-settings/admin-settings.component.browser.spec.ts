import { Fixtures } from 'src/testing/fixtures';
import { AgentStage } from 'src/app/agent/model/agent-stage.enum';
import { AgentEffort } from 'src/app/agent/model/agent-effort.enum';
import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { of } from 'rxjs';
import { SettingsApi } from 'src/app/core/settings/settings.api.service';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { VersionApi } from '../../api/version.api.service';
import { AdminSettingsComponent } from './admin-settings.component';

/**
 * The version row tells an operator which release the instance is running.
 * The template is trimmed to just that row so the assertions do not depend on
 * the settings form rendering.
 */
describe('AdminSettingsComponent — build version (browser)', () => {
    async function setup(buildInfo: { version: string; commit: string }) {
        await TestBed.configureTestingModule({
            imports: [ReactiveFormsModule],
            declarations: [AdminSettingsComponent],
            providers: [
                {
                    provide: SettingsApi,
                    useValue: {
                        load$: () =>
                            of({
                                tablePageSize: 50,
                                kanbanPageSize: 20,
                                ganttBacklogPageSize: 30,
                                agentStageDefaults: Fixtures.appAgentStageDefaults()
                            }),
                        update$: () => of({})
                    }
                },
                { provide: VersionApi, useValue: { load$: () => of(buildInfo) } }
            ]
        })
            .overrideComponent(AdminSettingsComponent, {
                set: { template: `<span class="version">{{ versionLabel() }}</span>` }
            })
            .compileComponents();

        const fixture = TestBed.createComponent(AdminSettingsComponent);
        fixture.detectChanges();
        return fixture.nativeElement.querySelector('.version') as HTMLElement;
    }

    it('shows the version with a short commit', async () => {
        // The workflow stamps the full 40-char SHA; the UI shows the short form.
        const row = await setup({
            version: '1.0.0',
            commit: 'abc1234def5678901234567890abcdef12345678'
        });

        expect(row.textContent?.trim()).toBe('1.0.0 (abc1234)');
    });

    it('shows a dev build without a commit suffix', async () => {
        const row = await setup({ version: 'dev', commit: 'unknown' });

        expect(row.textContent?.trim()).toBe('dev');
    });
});

/**
 * The thinking switch is a boolean end to end, so the form binds it straight
 * through — and it must save on the click, not on a blur a switch never gets.
 */
describe('AdminSettingsComponent — agent thinking switch (browser)', () => {
    const loaded = {
        tablePageSize: 50,
        kanbanPageSize: 20,
        ganttBacklogPageSize: 30,
        sprintVelocityLimit: 10,
        userApiKeyLimit: 10,
        isAgentThinkingPersisted: false,
        agentThinkingMaxKb: 1024,
        attachmentMaxSizeMb: 25,
        agentStageDefaults: Fixtures.appAgentStageDefaults()
    };

    async function setup() {
        const update$ = vi.fn().mockReturnValue(of({ ...loaded, isAgentThinkingPersisted: true }));
        await TestBed.configureTestingModule({
            imports: [ReactiveFormsModule],
            declarations: [AdminSettingsComponent],
            providers: [
                {
                    provide: SettingsApi,
                    useValue: { load$: () => of(loaded), update$ }
                },
                {
                    provide: VersionApi,
                    useValue: { load$: () => of({ version: 'dev', commit: 'unknown' }) }
                }
            ]
        })
            .overrideComponent(AdminSettingsComponent, {
                set: {
                    template: `<form [formGroup]="form"><input type="checkbox" class="switch" formControlName="isAgentThinkingPersisted" /></form>`
                }
            })
            .compileComponents();

        const fixture = TestBed.createComponent(AdminSettingsComponent);
        fixture.detectChanges();
        return {
            fixture,
            update$,
            input: fixture.nativeElement.querySelector('.switch') as HTMLInputElement
        };
    }

    it('reflects the stored value as a switch position', async () => {
        const { input } = await setup();

        expect(input.checked).toBe(false);
    });

    it('saves the flipped switch without waiting for a blur', async () => {
        const { fixture, update$, input } = await setup();

        input.click();
        fixture.detectChanges();

        expect(update$).toHaveBeenCalledTimes(1);
        expect(update$.mock.calls[0][0]).toMatchObject({
            isAgentThinkingPersisted: true
        });
    });
});

describe('AdminSettingsComponent — agent turn defaults (browser)', () => {
    const loaded = {
        tablePageSize: 50,
        kanbanPageSize: 20,
        ganttBacklogPageSize: 30,
        sprintVelocityLimit: 10,
        userApiKeyLimit: 10,
        isAgentThinkingPersisted: true,
        agentThinkingMaxKb: 1024,
        attachmentMaxSizeMb: 25,
        agentStageDefaults: Fixtures.appAgentStageDefaults()
    };

    it('saves a changed turn default on blur', async () => {
        const update$ = vi.fn().mockReturnValue(of(loaded));
        await TestBed.configureTestingModule({
            imports: [ReactiveFormsModule],
            declarations: [AdminSettingsComponent],
            providers: [
                { provide: SettingsApi, useValue: { load$: () => of(loaded), update$ } },
                {
                    provide: VersionApi,
                    useValue: { load$: () => of({ version: 'dev', commit: 'unknown' }) }
                }
            ]
        })
            .overrideComponent(AdminSettingsComponent, {
                set: {
                    template: `<form [formGroup]="form"><ng-container formGroupName="agentStageDefaults"><ng-container formGroupName="release_notes"><input type="number" class="turns" formControlName="maxTurns" /></ng-container></ng-container></form>`
                }
            })
            .compileComponents();
        const fixture = TestBed.createComponent(AdminSettingsComponent);
        fixture.detectChanges();
        const input = fixture.nativeElement.querySelector('.turns') as HTMLInputElement;

        expect(input.value).toBe('250');
        input.value = '60';
        input.dispatchEvent(new Event('input'));
        input.dispatchEvent(new Event('blur'));
        fixture.detectChanges();

        expect(update$).toHaveBeenCalledTimes(1);
        expect(update$.mock.calls[0][0]).toMatchObject({
            agentStageDefaults: {
                [AgentStage.ReleaseNotes]: { maxTurns: 60, effort: '' },
                [AgentStage.Implementation]: { maxTurns: 500, effort: '' }
            }
        });
    });
});

describe('AdminSettingsComponent — wiki always token limit (browser)', () => {
    const loaded = {
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

    it('saves a changed limit on blur', async () => {
        const update$ = vi.fn().mockReturnValue(of({ ...loaded, wikiAlwaysTokenLimit: 8000 }));
        await TestBed.configureTestingModule({
            imports: [ReactiveFormsModule],
            declarations: [AdminSettingsComponent],
            providers: [
                { provide: SettingsApi, useValue: { load$: () => of(loaded), update$ } },
                {
                    provide: VersionApi,
                    useValue: { load$: () => of({ version: 'dev', commit: 'unknown' }) }
                }
            ]
        })
            .overrideComponent(AdminSettingsComponent, {
                set: {
                    template: `<form [formGroup]="form"><input class="limit" type="number" formControlName="wikiAlwaysTokenLimit" /></form>`
                }
            })
            .compileComponents();
        const fixture = TestBed.createComponent(AdminSettingsComponent);
        fixture.detectChanges();
        const input = fixture.nativeElement.querySelector('.limit') as HTMLInputElement;

        expect(input.value).toBe('12000');
        input.value = '8000';
        input.dispatchEvent(new Event('input'));
        input.dispatchEvent(new Event('blur'));
        fixture.detectChanges();

        expect(update$).toHaveBeenCalledTimes(1);
        expect(update$.mock.calls[0][0]).toMatchObject({ wikiAlwaysTokenLimit: 8000 });
    });
});

describe('AdminSettingsComponent — plantuml server url (browser)', () => {
    const loaded = {
        tablePageSize: 50,
        kanbanPageSize: 20,
        ganttBacklogPageSize: 30,
        sprintVelocityLimit: 10,
        userApiKeyLimit: 10,
        isAgentThinkingPersisted: true,
        agentThinkingMaxKb: 1024,
        attachmentMaxSizeMb: 25,
        agentStageDefaults: Fixtures.appAgentStageDefaults(),
        plantUmlServerUrl: 'http://puml:8080'
    };

    async function setup(update$ = vi.fn().mockReturnValue(of(loaded))) {
        await TestBed.configureTestingModule({
            imports: [ReactiveFormsModule],
            declarations: [AdminSettingsComponent],
            providers: [
                { provide: SettingsApi, useValue: { load$: () => of(loaded), update$ } },
                {
                    provide: VersionApi,
                    useValue: { load$: () => of({ version: 'dev', commit: 'unknown' }) }
                }
            ]
        })
            .overrideComponent(AdminSettingsComponent, {
                set: {
                    template: `<form [formGroup]="form"><input class="url" type="url" formControlName="plantUmlServerUrl" /></form>`
                }
            })
            .compileComponents();
        const fixture = TestBed.createComponent(AdminSettingsComponent);
        fixture.detectChanges();
        return {
            fixture,
            update$,
            input: fixture.nativeElement.querySelector('.url') as HTMLInputElement,
            store: TestBed.inject(SettingsStore)
        };
    }

    it('loads the stored url into the field', async () => {
        const { input } = await setup();
        expect(input.value).toBe('http://puml:8080');
    });

    it('saves the edited url on blur', async () => {
        const { fixture, update$, input } = await setup();

        input.value = 'http://other:8080';
        input.dispatchEvent(new Event('input'));
        input.dispatchEvent(new Event('blur'));
        fixture.detectChanges();

        expect(update$).toHaveBeenCalledTimes(1);
        expect(update$.mock.calls[0][0]).toMatchObject({ plantUmlServerUrl: 'http://other:8080' });
    });

    it('applies every saved url to SettingsStore without a reload', async () => {
        const update$ = vi
            .fn()
            .mockReturnValueOnce(of({ ...loaded, plantUmlServerUrl: 'http://saved:8080' }))
            .mockReturnValueOnce(of({ ...loaded, plantUmlServerUrl: '' }));
        const { fixture, input, store } = await setup(update$);
        expect(store.isPlantUmlEnabled()).toBe(false);

        input.value = 'http://saved:8080';
        input.dispatchEvent(new Event('input'));
        input.dispatchEvent(new Event('blur'));
        fixture.detectChanges();
        expect(store.isPlantUmlEnabled()).toBe(true);

        input.value = '';
        input.dispatchEvent(new Event('input'));
        input.dispatchEvent(new Event('blur'));
        fixture.detectChanges();
        expect(store.isPlantUmlEnabled()).toBe(false);
    });
});

describe('AdminSettingsComponent — agent thinking effort (browser)', () => {
    const loaded = {
        tablePageSize: 50,
        kanbanPageSize: 20,
        ganttBacklogPageSize: 30,
        sprintVelocityLimit: 10,
        userApiKeyLimit: 10,
        isAgentThinkingPersisted: true,
        agentThinkingMaxKb: 1024,
        attachmentMaxSizeMb: 25,
        agentStageDefaults: Fixtures.appAgentStageDefaults({
            [AgentStage.Design]: { maxTurns: 250, effort: AgentEffort.High }
        }),
        wikiAlwaysTokenLimit: 12000,
        plantUmlServerUrl: ''
    };

    async function setup(update$: ReturnType<typeof vi.fn>) {
        await TestBed.configureTestingModule({
            imports: [ReactiveFormsModule],
            declarations: [AdminSettingsComponent],
            providers: [
                { provide: SettingsApi, useValue: { load$: () => of(loaded), update$ } },
                {
                    provide: VersionApi,
                    useValue: { load$: () => of({ version: 'dev', commit: 'unknown' }) }
                }
            ]
        })
            .overrideComponent(AdminSettingsComponent, { set: { template: '' } })
            .compileComponents();
        const fixture = TestBed.createComponent(AdminSettingsComponent);
        fixture.detectChanges();
        return (fixture.componentInstance as unknown as { form: AdminSettingsComponent['form'] })
            .form;
    }

    it('saves a picked effort right away', async () => {
        const update$ = vi.fn().mockReturnValue(of(loaded));
        const form = await setup(update$);

        form.controls.agentStageDefaults.controls[AgentStage.ReleaseNotes].controls.effort.setValue(
            AgentEffort.Low
        );

        expect(update$).toHaveBeenCalledTimes(1);
        expect(update$.mock.calls[0][0]).toMatchObject({
            agentStageDefaults: {
                [AgentStage.Design]: { effort: 'high' },
                [AgentStage.ReleaseNotes]: { effort: 'low' }
            }
        });
    });

    it('a cleared effort is saved as the model default', async () => {
        const update$ = vi.fn().mockReturnValue(of(loaded));
        const form = await setup(update$);

        form.controls.agentStageDefaults.controls[AgentStage.Design].controls.effort.setValue(null);

        expect(update$.mock.calls[0][0]).toMatchObject({
            agentStageDefaults: { [AgentStage.Design]: { effort: '' } }
        });
    });
});

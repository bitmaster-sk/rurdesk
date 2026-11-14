import { Fixtures } from 'src/testing/fixtures';
import { AgentStage } from 'src/app/agent/model/agent-stage.enum';
import { AgentEffort } from 'src/app/agent/model/agent-effort.enum';
import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { of } from 'rxjs';
import { SettingsApi } from 'src/app/core/settings/settings.api.service';
import { AdminSettingsAgentsComponent } from './admin-settings-agents.component';

/**
 * The thinking switch is a boolean end to end, so the form binds it straight
 * through — and it must save on the click, not on a blur a switch never gets.
 */
describe('AdminSettingsAgentsComponent — agent thinking switch (browser)', () => {
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
            declarations: [AdminSettingsAgentsComponent],
            providers: [{ provide: SettingsApi, useValue: { load$: () => of(loaded), update$ } }]
        })
            .overrideComponent(AdminSettingsAgentsComponent, {
                set: {
                    template: `<form [formGroup]="form"><input type="checkbox" class="switch" formControlName="isAgentThinkingPersisted" /></form>`
                }
            })
            .compileComponents();

        const fixture = TestBed.createComponent(AdminSettingsAgentsComponent);
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

describe('AdminSettingsAgentsComponent — agent turn defaults (browser)', () => {
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
            declarations: [AdminSettingsAgentsComponent],
            providers: [{ provide: SettingsApi, useValue: { load$: () => of(loaded), update$ } }]
        })
            .overrideComponent(AdminSettingsAgentsComponent, {
                set: {
                    template: `<form [formGroup]="form"><ng-container formGroupName="agentStageDefaults"><ng-container formGroupName="release_notes"><input type="number" class="turns" formControlName="maxTurns" /></ng-container></ng-container></form>`
                }
            })
            .compileComponents();
        const fixture = TestBed.createComponent(AdminSettingsAgentsComponent);
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
        expect(update$.mock.calls[0][0]).not.toHaveProperty('tablePageSize');
    });
});

describe('AdminSettingsAgentsComponent — wiki always token limit (browser)', () => {
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
            declarations: [AdminSettingsAgentsComponent],
            providers: [{ provide: SettingsApi, useValue: { load$: () => of(loaded), update$ } }]
        })
            .overrideComponent(AdminSettingsAgentsComponent, {
                set: {
                    template: `<form [formGroup]="form"><input class="limit" type="number" formControlName="wikiAlwaysTokenLimit" /></form>`
                }
            })
            .compileComponents();
        const fixture = TestBed.createComponent(AdminSettingsAgentsComponent);
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

describe('AdminSettingsAgentsComponent — agent thinking effort (browser)', () => {
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
            declarations: [AdminSettingsAgentsComponent],
            providers: [{ provide: SettingsApi, useValue: { load$: () => of(loaded), update$ } }]
        })
            .overrideComponent(AdminSettingsAgentsComponent, { set: { template: '' } })
            .compileComponents();
        const fixture = TestBed.createComponent(AdminSettingsAgentsComponent);
        fixture.detectChanges();
        return (
            fixture.componentInstance as unknown as { form: AdminSettingsAgentsComponent['form'] }
        ).form;
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

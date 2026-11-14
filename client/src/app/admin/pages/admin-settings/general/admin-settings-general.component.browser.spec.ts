import { Fixtures } from 'src/testing/fixtures';
import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { of } from 'rxjs';
import { SettingsApi } from 'src/app/core/settings/settings.api.service';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { AdminSettingsGeneralComponent } from './admin-settings-general.component';

describe('AdminSettingsGeneralComponent — section fields (browser)', () => {
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

    it('saves a changed page size with only the general fields', async () => {
        const update$ = vi.fn().mockReturnValue(of(loaded));
        await TestBed.configureTestingModule({
            imports: [ReactiveFormsModule],
            declarations: [AdminSettingsGeneralComponent],
            providers: [{ provide: SettingsApi, useValue: { load$: () => of(loaded), update$ } }]
        })
            .overrideComponent(AdminSettingsGeneralComponent, {
                set: {
                    template: `<form [formGroup]="form"><input class="size" type="number" formControlName="tablePageSize" /></form>`
                }
            })
            .compileComponents();
        const fixture = TestBed.createComponent(AdminSettingsGeneralComponent);
        fixture.detectChanges();
        const input = fixture.nativeElement.querySelector('.size') as HTMLInputElement;

        input.value = '75';
        input.dispatchEvent(new Event('input'));
        input.dispatchEvent(new Event('blur'));
        fixture.detectChanges();

        expect(update$).toHaveBeenCalledTimes(1);
        expect(update$.mock.calls[0][0]).toMatchObject({ tablePageSize: 75 });
        expect(Object.keys(update$.mock.calls[0][0]).sort()).toEqual(
            [
                'attachmentMaxSizeMb',
                'ganttBacklogPageSize',
                'kanbanPageSize',
                'plantUmlServerUrl',
                'sprintVelocityLimit',
                'tablePageSize'
            ].sort()
        );
    });
});

describe('AdminSettingsGeneralComponent — plantuml server url (browser)', () => {
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
            declarations: [AdminSettingsGeneralComponent],
            providers: [{ provide: SettingsApi, useValue: { load$: () => of(loaded), update$ } }]
        })
            .overrideComponent(AdminSettingsGeneralComponent, {
                set: {
                    template: `<form [formGroup]="form"><input class="url" type="url" formControlName="plantUmlServerUrl" /></form>`
                }
            })
            .compileComponents();
        const fixture = TestBed.createComponent(AdminSettingsGeneralComponent);
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

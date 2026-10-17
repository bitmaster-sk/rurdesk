import { Injector, runInInjectionContext } from '@angular/core';
import { of } from 'rxjs';
import { type AppSettings } from './model/app-settings.model';
import { SettingsApi } from './settings.api.service';
import { SettingsStore } from './settings.store';

function build(getReturn?: AppSettings) {
    const load$ = vi.fn().mockReturnValue(of(getReturn));
    const injector = Injector.create({
        providers: [{ provide: SettingsApi, useValue: { load$ } }]
    });
    const store = runInInjectionContext(injector, () => new SettingsStore());
    return { store, load$ };
}

describe('SettingsStore', () => {
    it('falls back to defaults before load', () => {
        const { store } = build();
        expect(store.tablePageSize()).toBe(50);
        expect(store.kanbanPageSize()).toBe(20);
        expect(store.ganttBacklogPageSize()).toBe(30);
        expect(store.userApiKeyLimit()).toBe(10);
    });

    it('applies loaded values after load', () => {
        const { store } = build({
            tablePageSize: 75,
            kanbanPageSize: 30,
            ganttBacklogPageSize: 40,
            sprintVelocityLimit: 15,
            userApiKeyLimit: 25,
            isAgentThinkingPersisted: false,
            agentThinkingMaxKb: 2048,
            attachmentMaxSizeMb: 40,
            plantUmlServerUrl: 'http://puml:8080'
        });
        store.load();
        expect(store.tablePageSize()).toBe(75);
        expect(store.kanbanPageSize()).toBe(30);
        expect(store.ganttBacklogPageSize()).toBe(40);
        expect(store.userApiKeyLimit()).toBe(25);
    });

    it('enables plantuml only when a server url is configured', () => {
        const settings: AppSettings = {
            tablePageSize: 50,
            kanbanPageSize: 20,
            ganttBacklogPageSize: 30,
            sprintVelocityLimit: 10,
            userApiKeyLimit: 10,
            isAgentThinkingPersisted: true,
            agentThinkingMaxKb: 1024,
            attachmentMaxSizeMb: 25,
            plantUmlServerUrl: ''
        };
        const off = build(settings);
        off.store.load();
        expect(off.store.isPlantUmlEnabled()).toBe(false);

        const on = build({ ...settings, plantUmlServerUrl: 'http://puml:8080' });
        on.store.load();
        expect(on.store.isPlantUmlEnabled()).toBe(true);
    });
});

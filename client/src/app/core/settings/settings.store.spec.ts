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
            agentThinkingMaxKb: 2048
        });
        store.load();
        expect(store.tablePageSize()).toBe(75);
        expect(store.kanbanPageSize()).toBe(30);
        expect(store.ganttBacklogPageSize()).toBe(40);
        expect(store.userApiKeyLimit()).toBe(25);
    });
});

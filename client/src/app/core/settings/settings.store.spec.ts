import { of } from 'rxjs';
import type { SettingsApi, AppSettings } from './settings.api.service';
import { SettingsStore } from './settings.store';

function build(getReturn?: AppSettings) {
    const getSettings$ = vi.fn().mockReturnValue(of(getReturn));
    const store = new SettingsStore({ getSettings$ } as unknown as SettingsApi);
    return { store, getSettings$ };
}

describe('SettingsStore', () => {
    it('falls back to defaults before load', () => {
        const { store } = build();
        expect(store.tablePageSize()).toBe(50);
        expect(store.kanbanPageSize()).toBe(20);
        expect(store.ganttBacklogPageSize()).toBe(30);
    });

    it('applies loaded values after load', () => {
        const { store } = build({
            tablePageSize: 75,
            kanbanPageSize: 30,
            ganttBacklogPageSize: 40
        });
        store.load();
        expect(store.tablePageSize()).toBe(75);
        expect(store.kanbanPageSize()).toBe(30);
        expect(store.ganttBacklogPageSize()).toBe(40);
    });
});

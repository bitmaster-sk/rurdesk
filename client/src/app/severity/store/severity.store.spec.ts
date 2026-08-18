import { Injector, runInInjectionContext } from '@angular/core';
import { of } from 'rxjs';
import { SeverityStore } from './severity.store';
import { SeverityApi } from '../api/severity.api.service';
import { IssueSeverity } from '../model/issue-severity.model';

function latest<T>(obs: { subscribe: (fn: (v: T) => void) => unknown }): T {
    let v!: T;
    obs.subscribe(x => (v = x));
    return v;
}

const severities = [
    { idSeverity: 1, idProject: 1, title: 'Low' },
    { idSeverity: 2, idProject: 2, title: 'High' }
] as IssueSeverity[];

describe('SeverityStore', () => {
    function build(): SeverityStore {
        const injector = Injector.create({
            providers: [{ provide: SeverityApi, useValue: { load$: () => of(severities) } }]
        });
        const store = runInInjectionContext(injector, () => new SeverityStore());
        store.load();
        return store;
    }

    it('builds a lookup map of all severities', () => {
        expect(latest(build().severitiesMap$).size).toBe(2);
    });

    it('filters severities by project', () => {
        expect(latest(build().severitiesByProject$(1)).map(s => s.idSeverity)).toEqual([1]);
    });

    it('builds a per-project lookup map', () => {
        const map = latest(build().severitiesMapByProject$(1));
        expect(map.size).toBe(1);
        expect(map.get(1)?.title).toBe('Low');
    });
});

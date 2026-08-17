import { Injector, runInInjectionContext } from '@angular/core';
import { of } from 'rxjs';
import { StateStore } from './state.store';
import { StateApi } from '../api/state.api.service';
import { IssueState } from '../model/issue-state.model';

function latest<T>(obs: { subscribe: (fn: (v: T) => void) => unknown }): T {
    let v!: T;
    obs.subscribe(x => (v = x));
    return v;
}

const states = [
    { idState: 1, idProject: 1, name: 'Todo' },
    { idState: 2, idProject: 2, name: 'Done' }
] as IssueState[];

describe('StateStore', () => {
    function build(): StateStore {
        const injector = Injector.create({
            providers: [{ provide: StateApi, useValue: { load$: () => of(states) } }]
        });
        const store = runInInjectionContext(injector, () => new StateStore());
        store.load();
        return store;
    }

    it('filters states by project', () => {
        expect(latest(build().statesByProject$(1)).map(s => s.idState)).toEqual([1]);
    });

    it('builds a per-project lookup map', () => {
        const map = latest(build().statesMapByProject$(2));
        expect(map.size).toBe(1);
        expect(map.get(2)?.name).toBe('Done');
    });
});

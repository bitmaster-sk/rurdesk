import { Injector, runInInjectionContext } from '@angular/core';
import { of } from 'rxjs';
import { IssueTypeStore } from './issue-type.store';
import { IssueTypeApi } from '../api/issue-type.api.service';
import { IssueType } from '../model/issue-type.model';

const issueTypes = [
    { idIssueType: 1, idProject: 1, name: 'Bug', protected: false, orderRank: 1 },
    { idIssueType: 2, idProject: 1, name: 'Feature', protected: false, orderRank: 2 },
    { idIssueType: 3, idProject: 2, name: 'Task', protected: false, orderRank: 1 }
] as IssueType[];

abstract class Emitted {
    public static latest<T>(obs: { subscribe: (fn: (v: T) => void) => unknown }): T {
        let value!: T;
        obs.subscribe(x => (value = x));
        return value;
    }
}

describe('IssueTypeStore', () => {
    function build(): IssueTypeStore {
        const injector = Injector.create({
            providers: [{ provide: IssueTypeApi, useValue: { load$: () => of(issueTypes) } }]
        });
        const store = runInInjectionContext(injector, () => new IssueTypeStore());
        store.load();
        return store;
    }

    it('builds a lookup map of all issue types', () => {
        expect(Emitted.latest(build().issueTypesMap$).size).toBe(3);
    });

    it('filters issue types by project', () => {
        expect(Emitted.latest(build().issueTypesByProject$(1)).map(t => t.idIssueType)).toEqual([
            1, 2
        ]);
    });

    it('builds a per-project lookup map', () => {
        const map = Emitted.latest(build().issueTypesMapByProject$(2));
        expect(map.size).toBe(1);
        expect(map.get(3)?.name).toBe('Task');
    });
});

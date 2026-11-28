import { Injector, runInInjectionContext } from '@angular/core';
import { NEVER, Subject, of } from 'rxjs';
import { Project } from './model/project.model';
import { ProjectApi } from './api/project.api.service';
import { ProjectStore } from './project.store';

const loaded: Project = {
    idProject: 10,
    name: 'Rurdesk',
    color: '#000',
    idStateDefault: 2,
    idSeverityDefault: 3,
    idIssueTypeDefault: 1
};

function buildStore(projectApi: ProjectApi): ProjectStore {
    const injector = Injector.create({
        providers: [{ provide: ProjectApi, useValue: projectApi }]
    });
    return runInInjectionContext(injector, () => new ProjectStore());
}

function latest<T>(observable: { subscribe: (onNext: (v: T) => void) => unknown }): T {
    let value!: T;
    observable.subscribe((v: T) => (value = v));
    return value;
}

describe('ProjectStore', () => {
    it('loads a project through the API and emits it on project$', () => {
        const loadOne$ = vi.fn().mockReturnValue(of(loaded));
        const store = buildStore({ loadOne$ } as unknown as ProjectApi);

        store.load(10);

        expect(loadOne$).toHaveBeenCalledWith(10);
        expect(latest(store.project$)).toBe(loaded);
    });

    it('does not replay the previous project while another one is loading', () => {
        const other: Project = { ...loaded, idProject: 11, name: 'Agent VL' };
        const pending$ = new Subject<Project>();
        const loadOne$ = vi.fn().mockReturnValueOnce(of(loaded)).mockReturnValueOnce(pending$);
        const store = buildStore({ loadOne$ } as unknown as ProjectApi);
        store.load(10);

        store.load(11);
        const seen: Project[] = [];
        store.project$.subscribe(project => seen.push(project));
        pending$.next(other);

        expect(seen).toEqual([other]);
    });

    it('keeps the project while the same one reloads', () => {
        const loadOne$ = vi.fn().mockReturnValueOnce(of(loaded)).mockReturnValueOnce(NEVER);
        const store = buildStore({ loadOne$ } as unknown as ProjectApi);
        store.load(10);

        store.load(10);

        expect(latest(store.project$)).toBe(loaded);
    });

    it('update() PATCHes through the API and re-emits the saved value', () => {
        const saved: Project = { ...loaded, idStateDefault: 5 };
        const update$ = vi.fn().mockReturnValue(of(saved));
        const store = buildStore({ update$ } as unknown as ProjectApi);

        latest(store.update({ ...loaded, idStateDefault: 5 }));

        expect(update$).toHaveBeenCalledWith({ ...loaded, idStateDefault: 5 });
        expect(latest(store.project$)).toBe(saved);
    });

    it('setProject() re-emits the project without any API call', () => {
        const loadOne$ = vi.fn();
        const update$ = vi.fn();
        const store = buildStore({ loadOne$, update$ } as unknown as ProjectApi);

        store.setProject(loaded);

        expect(latest(store.project$)).toEqual(loaded);
        expect(loadOne$).not.toHaveBeenCalled();
        expect(update$).not.toHaveBeenCalled();
    });
});

import { Injector, runInInjectionContext } from '@angular/core';
import { Subject } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { IssueApi } from 'src/app/issue/api/issue.api.service';
import { ProjectApi } from 'src/app/project/api/project.api.service';
import { Issue } from 'src/app/issue/model/issue.model';
import { Project } from 'src/app/project/model/project.model';
import { IssueLinkStore } from './issue-link.store';

const issue = (idIssuePublic: number, title: string): Issue => ({
    idIssue: idIssuePublic,
    idIssuePublic,
    idProject: 7,
    idState: null,
    idSeverity: null,
    idIssueType: null,
    title,
    description: '',
    tracked: 0
});

const project = (idProject: number, name: string): Project => ({
    idProject,
    name,
    color: ''
});

function buildStore(): {
    store: IssueLinkStore;
    load: ReturnType<typeof vi.fn>;
    loadProjects: ReturnType<typeof vi.fn>;
} {
    const load = vi.fn().mockImplementation(() => new Subject<Issue[]>());
    const loadProjects = vi.fn().mockImplementation(() => new Subject<Project[]>());
    const injector = Injector.create({
        providers: [
            { provide: IssueApi, useValue: { load$: load } },
            { provide: ProjectApi, useValue: { load$: loadProjects } }
        ]
    });
    return {
        store: runInInjectionContext(injector, () => new IssueLinkStore()),
        load,
        loadProjects
    };
}

describe('IssueLinkStore', () => {
    it('loads the list on first use and serves it from the cache', () => {
        const { store, load } = buildStore();
        store.ensure(7);
        expect(load).toHaveBeenCalledWith({
            idProject: 7,
            orderColumn: 'createAt',
            orderDirection: 'desc'
        });
        expect(store.list(7)).toBeNull();

        load.mock.results[0].value.next([issue(1, 'Fix login')]);
        expect(store.list(7)).toEqual([{ idIssuePublic: 1, idProject: 7, title: 'Fix login' }]);
        expect(store.list(8)).toBeNull();
    });

    it('does not start a second request while the first one is in flight', () => {
        const { store, load } = buildStore();
        store.ensure(7);
        store.ensure(7);
        store.ensure(7);
        expect(load).toHaveBeenCalledTimes(1);
    });

    it('refreshes the list in the background on a later ensure, keeping the old list meanwhile', () => {
        const { store, load } = buildStore();
        store.ensure(7);
        load.mock.results[0].value.next([issue(1, 'Fix login')]);

        store.ensure(7);
        expect(load).toHaveBeenCalledTimes(2);
        expect(store.list(7)).toEqual([{ idIssuePublic: 1, idProject: 7, title: 'Fix login' }]);

        load.mock.results[1].value.next([issue(1, 'Fix login'), issue(2, 'Add dark mode')]);
        expect(store.list(7)).toEqual([
            { idIssuePublic: 1, idProject: 7, title: 'Fix login' },
            { idIssuePublic: 2, idProject: 7, title: 'Add dark mode' }
        ]);
    });

    it('allows a retry after a failed load', () => {
        const { store, load } = buildStore();
        store.ensure(7);
        load.mock.results[0].value.error(new Error('boom'));

        store.ensure(7);
        expect(load).toHaveBeenCalledTimes(2);
        expect(store.list(7)).toBeNull();
    });

    it('keeps projects separate', () => {
        const { store, load } = buildStore();
        store.ensure(7);
        store.ensure(9);
        expect(load).toHaveBeenCalledTimes(2);
        load.mock.results[0].value.next([issue(1, 'Fix login')]);
        expect(store.list(9)).toBeNull();
    });

    it('loads every visible project into the all-projects list with project names', () => {
        const { store, load, loadProjects } = buildStore();
        store.ensureAll();
        expect(loadProjects).toHaveBeenCalledTimes(1);
        expect(store.listAll()).toBeNull();

        loadProjects.mock.results[0].value.next([project(7, 'Rurdesk'), project(9, 'Website')]);
        expect(load.mock.calls.map(call => call[0].idProject)).toEqual([7, 9]);
        expect(store.listAll()).toBeNull();

        load.mock.results[0].value.next([issue(1, 'Fix login')]);
        expect(store.listAll()).toBeNull();

        load.mock.results[1].value.next([issue(12, 'Add dark mode')]);
        expect(store.listAll()).toEqual([
            { idIssuePublic: 1, idProject: 7, projectName: 'Rurdesk', title: 'Fix login' },
            { idIssuePublic: 12, idProject: 9, projectName: 'Website', title: 'Add dark mode' }
        ]);
    });

    it('serves an empty all-projects list when the user has no projects', () => {
        const { store, load, loadProjects } = buildStore();
        store.ensureAll();
        loadProjects.mock.results[0].value.next([]);
        expect(load).not.toHaveBeenCalled();
        expect(store.listAll()).toEqual([]);
    });

    it('does not start a second all-projects load while the first one is in flight', () => {
        const { store, loadProjects } = buildStore();
        store.ensureAll();
        store.ensureAll();
        expect(loadProjects).toHaveBeenCalledTimes(1);
    });

    it('refreshes the all-projects list in the background on a later ensureAll, keeping the old list meanwhile', () => {
        const { store, load, loadProjects } = buildStore();
        store.ensureAll();
        loadProjects.mock.results[0].value.next([project(7, 'Rurdesk')]);
        load.mock.results[0].value.next([issue(1, 'Fix login')]);
        expect(store.listAll()).toEqual([
            { idIssuePublic: 1, idProject: 7, projectName: 'Rurdesk', title: 'Fix login' }
        ]);

        store.ensureAll();
        expect(loadProjects).toHaveBeenCalledTimes(2);
        expect(store.listAll()).toEqual([
            { idIssuePublic: 1, idProject: 7, projectName: 'Rurdesk', title: 'Fix login' }
        ]);

        loadProjects.mock.results[1].value.next([project(7, 'Rurdesk')]);
        load.mock.results[1].value.next([issue(1, 'Fix login'), issue(2, 'New task')]);
        expect(store.listAll()).toEqual([
            { idIssuePublic: 1, idProject: 7, projectName: 'Rurdesk', title: 'Fix login' },
            { idIssuePublic: 2, idProject: 7, projectName: 'Rurdesk', title: 'New task' }
        ]);
    });

    it('allows a retry of the all-projects load after a failure', () => {
        const { store, loadProjects } = buildStore();
        store.ensureAll();
        loadProjects.mock.results[0].value.error(new Error('boom'));
        expect(store.listAll()).toBeNull();

        store.ensureAll();
        expect(loadProjects).toHaveBeenCalledTimes(2);
    });

    it('allows a retry when one project load fails', () => {
        const { store, load, loadProjects } = buildStore();
        store.ensureAll();
        loadProjects.mock.results[0].value.next([project(7, 'Rurdesk'), project(9, 'Website')]);
        load.mock.results[0].value.next([issue(1, 'Fix login')]);
        load.mock.results[1].value.error(new Error('boom'));
        expect(store.listAll()).toBeNull();

        store.ensureAll();
        loadProjects.mock.results[1].value.next([project(7, 'Rurdesk'), project(9, 'Website')]);
        load.mock.results[2].value.next([issue(1, 'Fix login')]);
        load.mock.results[3].value.next([issue(12, 'Add dark mode')]);
        expect(store.listAll()).toEqual([
            { idIssuePublic: 1, idProject: 7, projectName: 'Rurdesk', title: 'Fix login' },
            { idIssuePublic: 12, idProject: 9, projectName: 'Website', title: 'Add dark mode' }
        ]);
    });
});

import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { Subject, of } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ProjectApi } from 'src/app/project/api/project.api.service';
import { Project } from 'src/app/project/model/project.model';
import { ProjectStore } from 'src/app/project/project.store';
import { ProjectContextStore } from './project-context.store';

@Component({ selector: 'app-page-stub', template: '', standalone: true })
class PageStub {}

const project = (idProject: number, name: string): Project => ({ idProject, name, color: '' });

const PROJECTS = [project(1, 'Alpha'), project(2, 'Beta'), project(3, 'Gamma')];

function setup(
    storedId: string | null,
    projects: Project[] = PROJECTS,
    storedRecent: string | null = null
): { store: ProjectContextStore; router: Router; project$: Subject<Project> } {
    if (storedId === null) {
        localStorage.removeItem('lastProjectId');
    } else {
        localStorage.setItem('lastProjectId', storedId);
    }
    if (storedRecent === null) {
        localStorage.removeItem('recentProjectIds');
    } else {
        localStorage.setItem('recentProjectIds', storedRecent);
    }
    const project$ = new Subject<Project>();
    TestBed.configureTestingModule({
        providers: [
            provideRouter([{ path: '**', component: PageStub }]),
            { provide: ProjectApi, useValue: { load$: () => of(projects) } },
            { provide: ProjectStore, useValue: { project$ } }
        ]
    });
    return {
        store: TestBed.inject(ProjectContextStore),
        router: TestBed.inject(Router),
        project$
    };
}

describe('ProjectContextStore', () => {
    afterEach(() => {
        localStorage.removeItem('lastProjectId');
        localStorage.removeItem('recentProjectIds');
    });

    it('uses the project from the URL on project pages', async () => {
        const { store, router } = setup('2');
        store.load();
        await router.navigateByUrl('/project/3/issue/view/table');

        expect(store.idProject()).toBe(3);
        expect(store.isOnProjectRoute()).toBe(true);
    });

    it('keeps the last visited project on a global page', async () => {
        const { store, router } = setup(null);
        store.load();
        await router.navigateByUrl('/project/3/view');
        await router.navigateByUrl('/message/1/team/view');

        expect(store.idProject()).toBe(3);
        expect(store.isOnProjectRoute()).toBe(false);
        expect(store.project()?.name).toBe('Gamma');
        expect(localStorage.getItem('lastProjectId')).toBe('3');
    });

    it('uses the stored project on a fresh load of a global page', async () => {
        const { store, router } = setup('2');
        await router.navigateByUrl('/agent-chat');
        store.load();

        expect(store.idProject()).toBe(2);
    });

    it('forgets a stored project that no longer exists and falls back to the first one', async () => {
        const { store, router } = setup('9');
        await router.navigateByUrl('/agent-chat');
        store.load();

        expect(store.idProject()).toBe(1);
        expect(localStorage.getItem('lastProjectId')).toBeNull();
    });

    it('has no project when the user has none', async () => {
        const { store, router } = setup(null, []);
        await router.navigateByUrl('/user');
        store.load();

        expect(store.idProject()).toBeNull();
        expect(store.project()).toBeNull();
    });

    it('opens the last project when the app starts on the root', async () => {
        const { store, router } = setup('2');
        await router.navigateByUrl('/');
        store.load();

        await vi.waitFor(() => expect(router.url).toBe('/project/2/view'));
    });

    it('keeps a renamed project in the list, sorted by name', async () => {
        const { store, router, project$ } = setup('1');
        await router.navigateByUrl('/project/1/view');
        store.load();

        project$.next(project(1, 'Zeta'));

        expect(store.project()?.name).toBe('Zeta');
        expect(store.projects().map(p => p.name)).toEqual(['Beta', 'Gamma', 'Zeta']);
    });

    it('lists the three most recently visited projects, newest first', async () => {
        const { store, router } = setup(null, [...PROJECTS, project(4, 'Delta')]);
        store.load();
        for (const id of [1, 2, 3, 2, 4]) {
            await router.navigateByUrl(`/project/${id}/view`);
        }

        expect(store.recent().map(p => p.idProject)).toEqual([4, 2, 3]);
        expect(localStorage.getItem('recentProjectIds')).toBe('[4,2,3]');
    });

    it('starts the recent list from the last visited project', async () => {
        const { store, router } = setup('2');
        await router.navigateByUrl('/agent-chat');
        store.load();

        expect(store.recent().map(p => p.idProject)).toEqual([2]);
    });

    it('leaves deleted projects out of the recent list', async () => {
        const { store, router } = setup('3', PROJECTS, '[9,3]');
        await router.navigateByUrl('/agent-chat');
        store.load();

        expect(store.recent().map(p => p.idProject)).toEqual([3]);
    });
});

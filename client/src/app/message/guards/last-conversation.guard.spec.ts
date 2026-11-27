import { Injector, runInInjectionContext, signal } from '@angular/core';
import { ActivatedRouteSnapshot, Router, RouterStateSnapshot } from '@angular/router';
import { Observable, firstValueFrom, isObservable, of } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ProjectContextStore } from 'src/app/home/store/project-context.store';
import { ProjectApi } from 'src/app/project/api/project.api.service';
import { Project } from 'src/app/project/model/project.model';
import { LastConversationGuard } from './last-conversation.guard';

/** The URL the guard sends `/message` to. */
async function target(idContextProject: number | null, projects: Project[] = []): Promise<string> {
    const injector = Injector.create({
        providers: [
            { provide: Router, useValue: { parseUrl: (url: string) => url } },
            {
                provide: ProjectContextStore,
                useValue: { idProject: signal(idContextProject) }
            },
            { provide: ProjectApi, useValue: { load$: () => of(projects) } }
        ]
    });
    const result = runInInjectionContext(injector, () =>
        LastConversationGuard.canActivate(
            {} as unknown as ActivatedRouteSnapshot,
            {} as unknown as RouterStateSnapshot
        )
    ) as unknown as string | Observable<string>;
    return isObservable(result) ? firstValueFrom(result) : result;
}

describe('LastConversationGuard', () => {
    let saved: Map<string, string>;

    beforeEach(() => {
        saved = new Map();
        vi.stubGlobal('localStorage', {
            getItem: (key: string) => saved.get(key) ?? null,
            setItem: (key: string, value: string) => saved.set(key, value)
        });
    });

    afterEach(() => vi.unstubAllGlobals());

    it('opens the chat the user opened last', async () => {
        LastConversationGuard.save('/message/12/1/view');

        expect(await target(4)).toBe('/message/12/1/view');
    });

    it("opens the side menu's project chat without a saved one", async () => {
        expect(await target(4)).toBe('/message/4/3/view');
    });

    it('opens the first project chat without any project in context', async () => {
        expect(await target(null, [{ idProject: 9, name: 'Apollo', color: '' }])).toBe(
            '/message/9/3/view'
        );
        expect(await target(null)).toBe('/');
    });

    it('falls through when the storage is broken or holds something else', async () => {
        saved.set('message.lastConversation', 'javascript:alert(1)');
        expect(await target(4)).toBe('/message/4/3/view');

        vi.stubGlobal('localStorage', {
            getItem: () => {
                throw new Error('denied');
            },
            setItem: () => {
                throw new Error('denied');
            }
        });
        LastConversationGuard.save('/message/12/1/view');
        expect(await target(4)).toBe('/message/4/3/view');
    });
});

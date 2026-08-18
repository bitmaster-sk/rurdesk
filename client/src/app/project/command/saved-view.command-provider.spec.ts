import { Injector } from '@angular/core';
import { Router } from '@angular/router';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { firstValueFrom, of, throwError } from 'rxjs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { CommandContext } from '../../core/command/command.model';
import { SavedViewApi } from '../api/saved-view.api.service';
import { SavedView } from '../model/saved-view.model';
import { IssueViewMode } from '../../issue/constants/issue-view-modes.enum';
import { SavedViewStore } from '../store/saved-view.store';
import { SavedViewCommandProvider } from './saved-view.command-provider';

function view(over: Partial<SavedView> = {}): SavedView {
    return {
        idSavedView: 7,
        idProject: 1,
        name: 'My bugs',
        viewType: IssueViewMode.TABLE,
        isShared: false,
        createBy: 1,
        updateAt: '2026-08-01T00:00:00Z',
        config: { v: 1 },
        ...over
    };
}

const ctx = (idProject: number | null): CommandContext => ({ idProject, issue: null });

describe('SavedViewCommandProvider', () => {
    let loadByProject$: ReturnType<typeof vi.fn>;
    let navigate: ReturnType<typeof vi.fn>;
    let provider: SavedViewCommandProvider;
    let store: SavedViewStore;

    beforeEach(() => {
        loadByProject$ = vi.fn(() => of([view()]));
        navigate = vi.fn();
        const injector = Injector.create({
            providers: [
                { provide: SavedViewApi, useValue: { loadByProject$ } },
                { provide: SavedViewStore, useClass: SavedViewStore },
                { provide: Router, useValue: { navigate } },
                { provide: I18nService, useValue: { instant: (key: string) => key } },
                { provide: SavedViewCommandProvider, useClass: SavedViewCommandProvider }
            ]
        });
        provider = injector.get(SavedViewCommandProvider);
        store = injector.get(SavedViewStore);
    });

    // Why the store is root-scoped: the Views group must be there on the project overview,
    // without the task list ever having been visited.
    it('primes the store on a cold load so the palette has views', async () => {
        expect(store.views()).toEqual([]);

        await firstValueFrom(provider.prime(ctx(1)));

        expect(loadByProject$).toHaveBeenCalledWith(1);
        expect(provider.getCommands(ctx(1))).toHaveLength(1);
    });

    it('does not refetch when the store already holds that project', async () => {
        await firstValueFrom(provider.prime(ctx(1)));
        loadByProject$.mockClear();

        await firstValueFrom(provider.prime(ctx(1)));

        expect(loadByProject$).not.toHaveBeenCalled();
    });

    it('refetches after a failed load rather than trusting the empty list', async () => {
        loadByProject$.mockReturnValueOnce(throwError(() => new Error('boom')));
        await firstValueFrom(provider.prime(ctx(1)));
        expect(provider.getCommands(ctx(1))).toEqual([]);

        loadByProject$.mockReturnValue(of([view()]));
        await firstValueFrom(provider.prime(ctx(1)));

        expect(provider.getCommands(ctx(1))).toHaveLength(1);
    });

    it('refetches when the project changes', async () => {
        await firstValueFrom(provider.prime(ctx(1)));
        loadByProject$.mockClear();
        loadByProject$.mockReturnValue(of([view({ idSavedView: 8, idProject: 2 })]));

        await firstValueFrom(provider.prime(ctx(2)));

        expect(loadByProject$).toHaveBeenCalledWith(2);
        expect(store.views().map(item => item.idSavedView)).toEqual([8]);
    });

    it('primes nothing outside a project', async () => {
        await firstValueFrom(provider.prime(ctx(null)));

        expect(loadByProject$).not.toHaveBeenCalled();
        expect(provider.getCommands(ctx(null))).toEqual([]);
    });

    it('running a command navigates with the view query param', async () => {
        await firstValueFrom(provider.prime(ctx(1)));

        provider.getCommands(ctx(1))[0].run();

        expect(navigate).toHaveBeenCalledWith(['/project', 1, 'issue', 'view', 'table'], {
            queryParams: { view: 7 }
        });
    });
});

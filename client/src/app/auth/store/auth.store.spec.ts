import { Injector, runInInjectionContext } from '@angular/core';
import { of, throwError } from 'rxjs';
import { AuthApi } from '../api/auth.api.service';
import { User } from '../model/user.model';
import { AuthStore } from './auth.store';

const ALICE: User = { idUser: 1, name: 'Alice', email: 'a@a.sk', colorAvatarBg: '#111' };

function build(api: Partial<AuthApi>): AuthStore {
    const injector = Injector.create({ providers: [{ provide: AuthApi, useValue: api }] });
    return runInInjectionContext(injector, () => new AuthStore());
}

describe('AuthStore', () => {
    it('publishes no user before one is loaded', () => {
        expect(build({}).user()).toBeNull();
    });

    it('publishes the user returned by the api', () => {
        const store = build({ loadUser$: () => of(ALICE) });

        store.loadUser$().subscribe();

        expect(store.user()).toEqual(ALICE);
    });

    it('getUser throws before the user is loaded', () => {
        expect(() => build({}).getUser()).toThrow();
    });

    it('getUser returns the user once it is loaded', () => {
        const store = build({ loadUser$: () => of(ALICE) });

        store.loadUser$().subscribe();

        expect(store.getUser()).toEqual(ALICE);
    });

    it('publishes the updated user after an update', () => {
        const updated: User = { ...ALICE, name: 'Alice B', colorAvatarBg: '#222' };
        const store = build({ loadUser$: () => of(ALICE), updateUser$: () => of(updated) });
        store.loadUser$().subscribe();

        store.updateUser$('Alice B', '#222').subscribe();

        expect(store.user()).toEqual(updated);
    });

    it('keeps the published user when a reload fails', () => {
        let fails = false;
        const store = build({
            loadUser$: () => (fails ? throwError(() => new Error('boom')) : of(ALICE))
        });
        store.loadUser$().subscribe();

        fails = true;
        store.loadUser$().subscribe({ error: () => {} });

        expect(store.user()).toEqual(ALICE);
    });

    it('exposes the user without any writer', () => {
        const store = build({});

        expect('set' in store.user).toBe(false);
        expect('update' in store.user).toBe(false);
    });
});

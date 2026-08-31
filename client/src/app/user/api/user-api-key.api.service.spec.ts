import { HttpClient } from '@angular/common/http';
import { Injector, runInInjectionContext } from '@angular/core';
import { of } from 'rxjs';
import { UserApiKey } from '../model/user-api-key.model';
import { UserApiKeyApi } from './user-api-key.api.service';

const mockKey: UserApiKey = {
    idApiKey: 5,
    idUser: 1,
    name: 'laptop',
    createdAt: '2026-08-29T10:00:00Z',
    expiresAt: null,
    lastUsedAt: null
};

describe('UserApiKeyApi', () => {
    let get: ReturnType<typeof vi.fn>;
    let post: ReturnType<typeof vi.fn>;
    let del: ReturnType<typeof vi.fn>;
    let api: UserApiKeyApi;

    beforeEach(() => {
        get = vi.fn().mockReturnValue(of([mockKey]));
        post = vi.fn().mockReturnValue(of({ ...mockKey, rawKey: 'raw' }));
        del = vi.fn().mockReturnValue(of(undefined));
        const injector = Injector.create({
            providers: [{ provide: HttpClient, useValue: { get, post, delete: del } }]
        });
        api = runInInjectionContext(injector, () => new UserApiKeyApi());
    });

    it('load$ GETs /api/private/user/api-key', () => {
        let res: UserApiKey[] | undefined;
        api.load$().subscribe(r => (res = r));

        expect(get).toHaveBeenCalledWith('/api/private/user/api-key');
        expect(res).toEqual([mockKey]);
    });

    it('insert$ POSTs the name and expiry', () => {
        api.insert$({ name: 'ci', expiresAt: '2027-01-01T00:00:00Z' }).subscribe();

        expect(post).toHaveBeenCalledWith('/api/private/user/api-key', {
            name: 'ci',
            expiresAt: '2027-01-01T00:00:00Z'
        });
    });

    it('regenerate$ POSTs to the rotate path', () => {
        api.regenerate$(5).subscribe();

        expect(post).toHaveBeenCalledWith('/api/private/user/api-key/5/token', {});
    });

    it('revoke$ DELETEs the key by id', () => {
        api.revoke$(5).subscribe();

        expect(del).toHaveBeenCalledWith('/api/private/user/api-key/5');
    });
});

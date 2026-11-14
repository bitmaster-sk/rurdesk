import { HttpClient } from '@angular/common/http';
import { Injector, runInInjectionContext } from '@angular/core';
import { of } from 'rxjs';
import { SILENCE_ERROR_TOAST } from 'src/app/core/request-context';
import { Register } from '../model/register.model';
import { AuthApi } from './auth.api.service';

function build() {
    const http = {
        get: vi.fn().mockReturnValue(of({ idUser: 1 })),
        post: vi.fn().mockReturnValue(of({ token: 'jwt-123' })),
        put: vi.fn().mockReturnValue(of(undefined)),
        patch: vi.fn().mockReturnValue(of({ idUser: 1 })),
        delete: vi.fn().mockReturnValue(of(undefined))
    };
    const injector = Injector.create({ providers: [{ provide: HttpClient, useValue: http }] });
    return { api: runInInjectionContext(injector, () => new AuthApi()), http };
}

describe('AuthApi', () => {
    it('maps the login response to the bare token string', () => {
        const { api } = build();
        let token: string | undefined;

        api.login$('a@a.com', 'pw', false).subscribe(t => (token = t));

        expect(token).toBe('jwt-123');
    });

    it('posts the credentials to the public login endpoint', () => {
        const { api, http } = build();

        api.login$('a@a.com', 'pw', false).subscribe();

        expect(http.post).toHaveBeenCalledWith('/api/public/login', {
            email: 'a@a.com',
            password: 'pw',
            hasExtendedSessionLifetime: false
        });
    });

    it('forwards the extended session lifetime request', () => {
        const { api, http } = build();

        api.login$('a@a.com', 'pw', true).subscribe();

        expect(http.post).toHaveBeenCalledWith('/api/public/login', {
            email: 'a@a.com',
            password: 'pw',
            hasExtendedSessionLifetime: true
        });
    });

    it('posts the registration payload with the error toast silenced', () => {
        const { api, http } = build();
        const register: Register = { name: 'Admin', email: 'a@a.com', password: 'secret' };

        api.register$(register).subscribe();

        const [url, body, options] = http.post.mock.calls[0] as [
            string,
            Register,
            { context: { get: (token: unknown) => boolean } }
        ];
        expect(url).toBe('/api/public/register');
        expect(body).toEqual(register);
        expect(options.context.get(SILENCE_ERROR_TOAST)).toBe(true);
    });

    it('sends the current and the new password to the password endpoint', () => {
        const { api, http } = build();

        api.changePassword$('old', 'new').subscribe();

        expect(http.put.mock.calls[0][0]).toBe('/api/private/user/password');
        expect(http.put.mock.calls[0][1]).toEqual({
            currentPassword: 'old',
            newPassword: 'new'
        });
    });

    it('patches the name and the avatar colour on the current-user endpoint', () => {
        const { api, http } = build();

        api.updateUser$('New', '#abcdef').subscribe();

        expect(http.patch.mock.calls[0][0]).toBe('/api/private/user');
        expect(http.patch.mock.calls[0][1]).toEqual({ name: 'New', colorAvatarBg: '#abcdef' });
    });

    it('reads the current user from the private user endpoint', () => {
        const { api, http } = build();

        api.loadUser$().subscribe();

        expect(http.get).toHaveBeenCalledWith('/api/private/user');
    });

    it('drops the session on logout', () => {
        const { api, http } = build();

        api.logout$().subscribe();

        expect(http.delete).toHaveBeenCalledWith('/api/private/logout');
    });
    it('loads the sign-in methods without an error toast', () => {
        const { api, http } = build();

        api.loadMethods$().subscribe();

        const [url, options] = http.get.mock.calls[0];
        expect(url).toBe('/api/public/auth/methods');
        expect(options.context.get(SILENCE_ERROR_TOAST)).toBe(true);
    });

    it('exchanges the SSO code without an error toast', () => {
        const { api, http } = build();

        api.exchangeSso$('abc').subscribe();

        const [url, body, options] = http.post.mock.calls[0];
        expect(url).toBe('/api/public/sso/exchange');
        expect(body).toEqual({ code: 'abc' });
        expect(options.context.get(SILENCE_ERROR_TOAST)).toBe(true);
    });
});

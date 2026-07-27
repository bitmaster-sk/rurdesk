// @vitest-environment jsdom
import type { HttpClient } from '@angular/common/http';
import { of } from 'rxjs';
import { UserService } from './user.service';

function build(httpReturn: unknown = of({})) {
    const post = vi.fn().mockReturnValue(httpReturn);
    const service = new UserService({ post } as unknown as HttpClient);
    return { service, post };
}

describe('UserService auth-local token', () => {
    beforeEach(() => localStorage.clear());

    it('saves the token to localStorage', () => {
        const { service } = build();
        service.saveAuthLocal('tok');
        expect(localStorage.getItem('Authorization')).toBe('tok');
    });

    it('reports presence and reads the token back', () => {
        const { service } = build();
        expect(service.hasAuthLocal()).toBe(false);
        service.saveAuthLocal('tok');
        expect(service.hasAuthLocal()).toBe(true);
        expect(service.getAuthLocal()).toBe('tok');
    });

    it('deletes the token from localStorage', () => {
        const { service } = build();
        service.saveAuthLocal('tok');
        service.deleteAuthLocal();
        expect(localStorage.getItem('Authorization')).toBeNull();
    });

    // The token used to be mirrored into an Authorization cookie for the
    // WebSocket handshake. Cookies ignore the port and expired with the browser
    // session, so that copy silently diverged from localStorage and broke the
    // socket while REST kept working. localStorage is now the only store.
    it('does not mirror the token into a cookie', () => {
        const { service } = build();
        service.saveAuthLocal('tok');
        expect(document.cookie).not.toContain('tok');
    });
});

describe('UserService.login', () => {
    it('maps the response to the bare token string', () => {
        const { service } = build(of({ token: 'jwt-123' }));
        let token: string | undefined;
        service.login('a@a.com', 'pw').subscribe(t => (token = t));
        expect(token).toBe('jwt-123');
    });
});

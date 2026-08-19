// @vitest-environment jsdom
import { AuthTokenStore } from './auth-token.store';

describe('AuthTokenStore', () => {
    beforeEach(() => localStorage.clear());

    it('saves the token to localStorage', () => {
        new AuthTokenStore().saveToken('tok');
        expect(localStorage.getItem('Authorization')).toBe('tok');
    });

    it('reports presence and reads the token back', () => {
        const store = new AuthTokenStore();
        expect(store.hasToken()).toBe(false);
        store.saveToken('tok');
        expect(store.hasToken()).toBe(true);
        expect(store.getToken()).toBe('tok');
    });

    it('deletes the token from localStorage', () => {
        const store = new AuthTokenStore();
        store.saveToken('tok');
        store.clearToken();
        expect(localStorage.getItem('Authorization')).toBeNull();
    });

    // The token used to be mirrored into an Authorization cookie for the
    // WebSocket handshake. Cookies ignore the port and expired with the browser
    // session, so that copy silently diverged from localStorage and broke the
    // socket while REST kept working. localStorage is now the only store.
    it('does not mirror the token into a cookie', () => {
        new AuthTokenStore().saveToken('tok');
        expect(document.cookie).not.toContain('tok');
    });
});

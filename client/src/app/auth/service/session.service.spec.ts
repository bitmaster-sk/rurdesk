import { Injector, runInInjectionContext } from '@angular/core';
import { Router } from '@angular/router';
import { of, throwError } from 'rxjs';
import { SettingsStore } from '../../core/settings/settings.store';
import { LicenseStore } from '../../core/license/license.store';
import { NotificationStore } from '../../notification/store/notification.store';
import { AttachmentStore } from '../../shared/attachment/store/attachment.store';
import { AuthApi } from '../api/auth.api.service';
import { AuthTokenStore } from '../store/auth-token.store';
import { SessionService } from './session.service';

function build(logout$ = () => of(undefined)) {
    const navigate = vi.fn();
    const load = vi.fn();
    const loadLicense = vi.fn();
    const saveToken = vi.fn();
    const clearToken = vi.fn();
    const init = vi.fn();
    const revokeAll = vi.fn();
    const injector = Injector.create({
        providers: [
            { provide: Router, useValue: { navigate } },
            { provide: AuthApi, useValue: { logout$ } },
            { provide: AuthTokenStore, useValue: { saveToken, clearToken } },
            { provide: SettingsStore, useValue: { load } },
            { provide: LicenseStore, useValue: { load: loadLicense } },
            { provide: NotificationStore, useValue: { init } },
            { provide: AttachmentStore, useValue: { revokeAll } }
        ]
    });
    const session = runInInjectionContext(injector, () => new SessionService());
    return { session, navigate, load, loadLicense, saveToken, clearToken, init, revokeAll };
}

describe('SessionService.start', () => {
    it('stores the token, loads the auth-gated state and lands on the home page', () => {
        const { session, navigate, load, loadLicense, saveToken, init } = build();

        session.start('jwt-123');

        expect(saveToken).toHaveBeenCalledWith('jwt-123');
        expect(load).toHaveBeenCalled();
        expect(loadLicense).toHaveBeenCalled();
        expect(init).toHaveBeenCalled();
        expect(navigate).toHaveBeenCalledWith(['/']);
    });
});

describe('SessionService.end', () => {
    it('drops the token and stays put when no target is given', () => {
        const { session, navigate, clearToken } = build();

        session.end();

        expect(clearToken).toHaveBeenCalled();
        expect(navigate).not.toHaveBeenCalled();
    });

    it('drops the token and navigates to the given target', () => {
        const { session, navigate, clearToken } = build();

        session.end('/login');

        expect(clearToken).toHaveBeenCalled();
        expect(navigate).toHaveBeenCalledWith(['/login']);
    });

    it('forgets downloaded attachments so the next user cannot see them', () => {
        const { session, revokeAll } = build();

        session.end('/login');

        expect(revokeAll).toHaveBeenCalled();
    });

    it('drops the token even when the server rejects the logout', () => {
        const { session, navigate, clearToken } = build(() => throwError(() => new Error('500')));

        session.end('/login');

        expect(clearToken).toHaveBeenCalled();
        expect(navigate).toHaveBeenCalledWith(['/login']);
    });
});

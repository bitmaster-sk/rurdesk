import { TestBed } from '@angular/core/testing';
import { ActivatedRoute } from '@angular/router';
import { RouterTestingModule } from '@angular/router/testing';
import { TranslateModule } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { UiModule } from '../../ui/ui.module';
import { AuthApi } from '../api/auth.api.service';
import { SessionService } from '../service/session.service';
import { SsoCallbackPage } from './sso-callback.page';

describe('SsoCallbackPage (browser)', () => {
    let authApi: { exchangeSso$: ReturnType<typeof vi.fn> };
    let session: { start: ReturnType<typeof vi.fn> };

    async function setup(fragment: string | null) {
        authApi = { exchangeSso$: vi.fn() };
        session = { start: vi.fn() };
        await TestBed.configureTestingModule({
            declarations: [SsoCallbackPage],
            imports: [RouterTestingModule, TranslateModule.forRoot(), UiModule],
            providers: [
                { provide: AuthApi, useValue: authApi },
                { provide: SessionService, useValue: session },
                { provide: ActivatedRoute, useValue: { snapshot: { fragment } } }
            ]
        }).compileComponents();
    }

    function render() {
        const fixture = TestBed.createComponent(SsoCallbackPage);
        fixture.detectChanges();
        return fixture;
    }

    function query(testId: string): HTMLElement | null {
        return document.querySelector(`[data-testid="${testId}"]`);
    }

    it('exchanges the code and starts the session on the return URL', async () => {
        await setup('code=abc');
        authApi.exchangeSso$.mockReturnValue(of({ token: 'jwt-1', returnUrl: '/issue/5' }));

        render();

        expect(authApi.exchangeSso$).toHaveBeenCalledWith('abc');
        expect(session.start).toHaveBeenCalledWith('jwt-1', '/issue/5');
    });

    it('shows an error with a way back when the exchange fails', async () => {
        await setup('code=abc');
        authApi.exchangeSso$.mockReturnValue(throwError(() => new Error('422')));

        render();

        expect(session.start).not.toHaveBeenCalled();
        expect(query('sso-callback-error')?.textContent).toContain('LOGIN.SSO_EXCHANGE.FAILED');
        expect(query('sso-callback-back')?.getAttribute('href')).toBe('/login');
    });

    it('shows the error without calling the API when the code is missing', async () => {
        await setup(null);

        render();

        expect(authApi.exchangeSso$).not.toHaveBeenCalled();
        expect(query('sso-callback-error')).not.toBeNull();
    });
});

import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { RouterTestingModule } from '@angular/router/testing';
import { TranslateModule } from '@ngx-translate/core';
import { Subject, of, throwError } from 'rxjs';
import { UiModule } from '../../ui/ui.module';
import { AuthRobotStub } from 'src/testing/stubs';
import { AuthApi } from '../api/auth.api.service';
import { PASSWORD_ONLY } from '../constants/sso.constants';
import {
    IconBuilding,
    IconEye,
    IconEyeOff,
    TablerIconComponent,
    provideTablerIcons
} from '@tabler/icons-angular';
import { SessionService } from '../service/session.service';
import { AuthTokenStore } from '../store/auth-token.store';
import { LoginComponent } from './login.component';

describe('LoginComponent (browser)', () => {
    let authApi: { login$: ReturnType<typeof vi.fn>; loadMethods$: ReturnType<typeof vi.fn> };
    let tokenStore: { hasToken: ReturnType<typeof vi.fn> };
    let session: { start: ReturnType<typeof vi.fn>; end: ReturnType<typeof vi.fn> };

    function error(): HTMLElement | null {
        return document.querySelector('.login__error');
    }

    beforeEach(async () => {
        authApi = { login$: vi.fn(), loadMethods$: vi.fn().mockReturnValue(of(PASSWORD_ONLY)) };
        tokenStore = { hasToken: vi.fn().mockReturnValue(false) };
        session = { start: vi.fn(), end: vi.fn() };

        await TestBed.configureTestingModule({
            declarations: [LoginComponent],
            imports: [
                ReactiveFormsModule,
                RouterTestingModule,
                TranslateModule.forRoot(),
                UiModule,
                TablerIconComponent,
                AuthRobotStub
            ],
            providers: [
                { provide: AuthApi, useValue: authApi },
                { provide: AuthTokenStore, useValue: tokenStore },
                { provide: SessionService, useValue: session },
                provideTablerIcons({ IconBuilding, IconEye, IconEyeOff })
            ]
        }).compileComponents();
    });

    function setup() {
        const fixture = TestBed.createComponent(LoginComponent);
        fixture.detectChanges();
        return fixture;
    }

    it('shows an error message when login fails', () => {
        authApi.login$.mockReturnValue(throwError(() => new Error('401')));
        const fixture = setup();
        fixture.componentInstance.form.setValue({
            email: 'a@b.sk',
            password: 'secret',
            hasExtendedSessionLifetime: false
        });
        expect(error()).toBeNull();

        fixture.componentInstance.onLogin();
        fixture.detectChanges();

        expect(error()).not.toBeNull();
    });

    it('does not call login$ when the form is empty', () => {
        authApi.login$.mockReturnValue(of('a-token'));
        const fixture = setup();

        fixture.componentInstance.onLogin();

        expect(authApi.login$).not.toHaveBeenCalled();
    });

    it('does not show an error message on a successful login', () => {
        authApi.login$.mockReturnValue(of('a-token'));
        const fixture = setup();
        fixture.componentInstance.form.setValue({
            email: 'a@b.sk',
            password: 'secret',
            hasExtendedSessionLifetime: false
        });

        fixture.componentInstance.onLogin();
        fixture.detectChanges();

        expect(error()).toBeNull();
        expect(session.start).toHaveBeenCalledWith('a-token');
    });

    it('asks for a default session lifetime when the checkbox is untouched', () => {
        authApi.login$.mockReturnValue(of('a-token'));
        const fixture = setup();
        fixture.componentInstance.form.setValue({
            email: 'a@b.sk',
            password: 'secret',
            hasExtendedSessionLifetime: false
        });

        fixture.componentInstance.onLogin();

        expect(authApi.login$).toHaveBeenCalledWith('a@b.sk', 'secret', false);
    });

    it('asks for an extended session lifetime when the checkbox is ticked', () => {
        authApi.login$.mockReturnValue(of('a-token'));
        const fixture = setup();
        const checkbox = fixture.nativeElement.querySelector(
            '[data-testid="login-extended-session"]'
        ) as HTMLInputElement;

        checkbox.click();
        fixture.detectChanges();
        fixture.componentInstance.form.patchValue({ email: 'a@b.sk', password: 'secret' });
        fixture.componentInstance.onLogin();

        expect(authApi.login$).toHaveBeenCalledWith('a@b.sk', 'secret', true);
    });

    it('toggles the password field type without changing its value or submitting', () => {
        authApi.login$.mockReturnValue(of('a-token'));
        const fixture = setup();
        fixture.componentInstance.form.patchValue({ password: 'secret' });
        fixture.detectChanges();

        const input = fixture.nativeElement.querySelector('#password') as HTMLInputElement;
        const toggle = fixture.nativeElement.querySelector(
            '[data-testid="login-password-toggle"] button'
        ) as HTMLButtonElement | null;
        expect(toggle).not.toBeNull();

        expect(input.type).toBe('password');
        expect(input.value).toBe('secret');

        toggle!.click();
        fixture.detectChanges();

        expect(input.type).toBe('text');
        expect(input.value).toBe('secret');
        expect(authApi.login$).not.toHaveBeenCalled();

        toggle!.click();
        fixture.detectChanges();

        expect(input.type).toBe('password');
        expect(input.value).toBe('secret');
    });

    it('offers an accessible name that depends on the toggle state', () => {
        authApi.login$.mockReturnValue(of('a-token'));
        const fixture = setup();
        fixture.componentInstance.form.patchValue({ password: 'secret' });
        fixture.detectChanges();

        const toggle = fixture.nativeElement.querySelector(
            '[data-testid="login-password-toggle"] button'
        ) as HTMLButtonElement;

        expect(toggle.getAttribute('aria-label')).toBe('LOGIN.SHOW_PASSWORD');

        toggle.click();
        fixture.detectChanges();

        expect(toggle.getAttribute('aria-label')).toBe('LOGIN.HIDE_PASSWORD');
    });

    it('clears the error once the user edits the form', () => {
        authApi.login$.mockReturnValue(throwError(() => new Error('401')));
        const fixture = setup();
        fixture.componentInstance.form.setValue({
            email: 'a@b.sk',
            password: 'secret',
            hasExtendedSessionLifetime: false
        });
        fixture.componentInstance.onLogin();
        fixture.detectChanges();
        expect(error()).not.toBeNull();

        fixture.componentInstance.form.get('password')!.setValue('newpass');
        fixture.detectChanges();

        expect(error()).toBeNull();
    });
    function withSsoError(code: string): void {
        TestBed.overrideProvider(ActivatedRoute, {
            useValue: { queryParamMap: of(convertToParamMap({ ssoError: code })) }
        });
    }

    function query(testId: string): HTMLElement | null {
        return document.querySelector(`[data-testid="${testId}"]`);
    }

    function withSso(sso: { isEnforced: boolean; canStaySignedIn: boolean }): void {
        authApi.loadMethods$.mockReturnValue(
            of({ password: true, sso: { protocol: 'oidc', ...sso }, ssoUnavailable: false })
        );
    }

    function click(testId: string): void {
        (query(testId)?.querySelector('button') as HTMLButtonElement).click();
    }

    it('shows company sign-in alone and the password form on request', () => {
        withSso({ isEnforced: false, canStaySignedIn: false });
        const fixture = setup();

        expect(query('login-sso')?.classList).toContain('ui-button--primary');
        expect(document.querySelector('#password')).toBeNull();
        expect(query('login-use-password')?.textContent).toContain('LOGIN.USE_PASSWORD');

        click('login-use-password');
        fixture.detectChanges();

        expect(query('login-sso')).toBeNull();
        expect(document.querySelector('#password')).not.toBeNull();
        expect(query('login-submit')?.querySelector('button')?.classList).toContain(
            'ui-button--primary'
        );

        click('login-back-to-sso');
        fixture.detectChanges();

        expect(query('login-sso')).not.toBeNull();
        expect(document.querySelector('#password')).toBeNull();
    });

    it('puts stay signed in under company sign-in when SSO sessions can last long', () => {
        withSso({ isEnforced: false, canStaySignedIn: true });
        const fixture = setup();

        const link = query('login-sso') as HTMLAnchorElement;
        const checkbox = query('login-extended-session') as HTMLInputElement;
        expect(link.getAttribute('href')).toBe('/api/public/sso/start');
        expect(
            link.compareDocumentPosition(checkbox) & Node.DOCUMENT_POSITION_FOLLOWING
        ).toBeTruthy();

        checkbox.click();
        fixture.detectChanges();
        expect(link.getAttribute('href')).toBe('/api/public/sso/start?extended=true');
        expect(fixture.componentInstance.form.controls.hasExtendedSessionLifetime.value).toBe(true);
    });

    it('offers stay signed in only with the password when SSO sessions cannot last long', () => {
        withSso({ isEnforced: false, canStaySignedIn: false });
        const fixture = setup();

        expect(query('login-extended-session')).toBeNull();

        click('login-use-password');
        fixture.detectChanges();

        const checkbox = query('login-extended-session') as HTMLInputElement;
        const submit = query('login-submit') as HTMLElement;
        expect(checkbox.closest('form')).not.toBeNull();
        expect(
            submit.compareDocumentPosition(checkbox) & Node.DOCUMENT_POSITION_FOLLOWING
        ).toBeTruthy();
    });

    it('offers the password only as an emergency sign-in while SSO is enforced', () => {
        withSso({ isEnforced: true, canStaySignedIn: false });
        const fixture = setup();

        expect(query('login-use-password')?.textContent).toContain('LOGIN.EMERGENCY');
        expect(query('login-break-glass-hint')).toBeNull();

        click('login-use-password');
        fixture.detectChanges();

        expect(document.querySelector('#password')).not.toBeNull();
        expect(query('login-break-glass-hint')?.textContent).toContain('LOGIN.BREAK_GLASS_ONLY');
    });

    it('shows no company sign-in for password-only instances', () => {
        setup();

        expect(query('login-sso')).toBeNull();
        expect(query('login-sso-unavailable')).toBeNull();
        expect(query('login-use-password')).toBeNull();
        expect(query('login-back-to-sso')).toBeNull();
        const submit = query('login-submit')?.querySelector('button') as HTMLButtonElement;
        expect(submit.classList).toContain('ui-button--primary');
    });

    it('shows no sign-in method before the methods arrive', () => {
        authApi.loadMethods$.mockReturnValue(new Subject());
        setup();

        expect(document.querySelector('#email')).toBeNull();
        expect(query('login-sso')).toBeNull();
    });

    it('falls back to password-only when the methods cannot be loaded', () => {
        authApi.loadMethods$.mockReturnValue(throwError(() => new Error('500')));
        setup();

        expect(query('login-sso')).toBeNull();
    });

    it('explains that company sign-in is unavailable', () => {
        authApi.loadMethods$.mockReturnValue(
            of({ password: true, sso: null, ssoUnavailable: true })
        );
        setup();

        expect(query('login-sso-unavailable')?.textContent).toContain('LOGIN.SSO_UNAVAILABLE');
    });

    it('translates the SSO error from the query string', () => {
        withSsoError('account_not_provisioned');
        setup();

        expect(query('login-sso-error')?.textContent).toContain(
            'LOGIN.SSO_ERROR.ACCOUNT_NOT_PROVISIONED'
        );
    });
});

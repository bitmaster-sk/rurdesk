import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { RouterTestingModule } from '@angular/router/testing';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { Subject, of, throwError } from 'rxjs';
import { UiModule } from '../../ui/ui.module';
import { AuthRobotStub } from 'src/testing/stubs';
import en from 'src/assets/i18n/en.json';
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

    function query(testId: string): HTMLElement | null {
        return document.querySelector(`[data-testid="${testId}"]`);
    }

    function fieldInput(id: string): HTMLInputElement {
        return document.querySelector(`#${id}`) as HTMLInputElement;
    }

    /** Real input: focus the field, set its value and let the form hear the event. */
    function typeIn(input: HTMLInputElement, value: string): void {
        input.focus();
        input.value = value;
        input.dispatchEvent(new Event('input'));
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
        // Translations are wired here, not in beforeEach: TestBed.inject would
        // instantiate the module before the SSO tests can override providers.
        const translate = TestBed.inject(TranslateService);
        translate.setTranslation('en', en);
        translate.use('en');

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
        expect(error()?.textContent).toContain('Incorrect email or password');
        // A server failure is not a field error; the fields keep their values and stay valid.
        expect(query('login-email-error')).toBeNull();
        expect(query('login-password-error')).toBeNull();
    });

    it('starts with no validation messages and an e-mail-typed field', () => {
        setup();

        expect(query('login-email-error')).toBeNull();
        expect(query('login-password-error')).toBeNull();
        const email = fieldInput('email');
        const password = fieldInput('password');
        expect(email.type).toBe('email');
        expect(email.getAttribute('inputmode')).toBe('email');
        expect(email.getAttribute('autocomplete')).toBe('email');
        expect(password.getAttribute('autocomplete')).toBe('current-password');
        expect(email.getAttribute('aria-invalid')).toBeNull();
        expect(email.getAttribute('aria-describedby')).toBeNull();
        expect(password.getAttribute('aria-invalid')).toBeNull();
        expect(password.getAttribute('aria-describedby')).toBeNull();
    });

    it('explains a missing e-mail once the empty field loses focus', () => {
        const fixture = setup();
        const email = fieldInput('email');

        email.focus();
        email.blur();
        fixture.detectChanges();

        expect(query('login-email-error')?.textContent).toContain('Enter your e-mail address.');
        expect(email.getAttribute('aria-invalid')).toBe('true');
        expect(email.getAttribute('aria-describedby')).toBe('login-email-error');
    });

    it('explains an invalid e-mail format once the field loses focus', () => {
        const fixture = setup();
        const email = fieldInput('email');

        typeIn(email, 'not-an-email');
        email.blur();
        fixture.detectChanges();

        expect(query('login-email-error')?.textContent).toContain('Enter a valid e-mail address');
    });

    it('explains an e-mail that runs past the length limit', () => {
        const fixture = setup();
        const email = fieldInput('email');
        // 253 characters that still match the e-mail format, so only maxLength(250) fails.
        const tooLong = `${'a'.repeat(64)}@${'b'.repeat(63)}.${'c'.repeat(63)}.${'d'.repeat(60)}`;

        typeIn(email, tooLong);
        email.blur();
        fixture.detectChanges();

        expect(query('login-email-error')?.textContent).toContain(
            'The e-mail address is limited to 250 characters.'
        );
    });

    it('withdraws the e-mail message as soon as the value becomes valid', () => {
        const fixture = setup();
        const email = fieldInput('email');

        typeIn(email, 'not-an-email');
        email.blur();
        fixture.detectChanges();
        expect(query('login-email-error')).not.toBeNull();

        typeIn(email, 'user@example.com');
        fixture.detectChanges();

        expect(query('login-email-error')).toBeNull();
        expect(email.getAttribute('aria-invalid')).toBeNull();
        expect(email.getAttribute('aria-describedby')).toBeNull();
    });

    it('explains a missing password once the empty field loses focus', () => {
        const fixture = setup();
        const password = fieldInput('password');

        password.focus();
        password.blur();
        fixture.detectChanges();

        expect(query('login-password-error')?.textContent).toContain('Enter your password.');
        expect(password.getAttribute('aria-invalid')).toBe('true');
        expect(password.getAttribute('aria-describedby')).toBe('login-password-error');
    });

    it('explains a password shorter than the minimum', () => {
        const fixture = setup();
        const password = fieldInput('password');

        typeIn(password, '1234');
        password.blur();
        fixture.detectChanges();

        expect(query('login-password-error')?.textContent).toContain(
            'The password must be at least 5 characters.'
        );
    });

    it('explains a password longer than the limit', () => {
        const fixture = setup();
        const password = fieldInput('password');

        typeIn(password, 'x'.repeat(101));
        password.blur();
        fixture.detectChanges();

        expect(query('login-password-error')?.textContent).toContain(
            'The password is limited to 100 characters.'
        );
    });

    it('keeps submit blocked while the fields are invalid and frees it when they are fixed', () => {
        const fixture = setup();
        const email = fieldInput('email');
        const password = fieldInput('password');
        const submit = () => query('login-submit')?.querySelector('button') as HTMLButtonElement;

        typeIn(email, 'not-an-email');
        typeIn(password, '1234');
        email.blur();
        fixture.detectChanges();

        expect(submit().disabled).toBe(true);
        fixture.componentInstance.onLogin();
        expect(authApi.login$).not.toHaveBeenCalled();

        typeIn(email, 'user@example.com');
        typeIn(password, 'secret');
        fixture.detectChanges();

        expect(submit().disabled).toBe(false);
    });

    it('does not call login$ when the form is empty', () => {
        authApi.login$.mockReturnValue(of('a-token'));
        const fixture = setup();

        fixture.componentInstance.onLogin();
        fixture.detectChanges();

        expect(authApi.login$).not.toHaveBeenCalled();
        const submit = query('login-submit')?.querySelector('button') as HTMLButtonElement;
        expect(submit.classList).not.toContain('ui-button--loading');
        expect(submit.disabled).toBe(true);
        expect(fixture.componentInstance.isSubmitting()).toBe(false);
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

    it('shows waiting on the button and sends one request for repeated submits', () => {
        const response$ = new Subject<string>();
        authApi.login$.mockReturnValue(response$);
        const fixture = setup();
        fixture.componentInstance.form.setValue({
            email: 'a@b.sk',
            password: 'secret',
            hasExtendedSessionLifetime: true
        });
        const submit = () => query('login-submit')?.querySelector('button') as HTMLButtonElement;

        fixture.componentInstance.onLogin();
        fixture.detectChanges();

        expect(authApi.login$).toHaveBeenCalledTimes(1);
        expect(authApi.login$).toHaveBeenCalledWith('a@b.sk', 'secret', true);
        expect(submit().classList).toContain('ui-button--loading');
        expect(submit().disabled).toBe(true);

        // A disabled button swallows clicks, but Enter still fires ngSubmit and
        // the handler can be re-invoked — the in-flight guard is what keeps
        // these at one request.
        (fixture.nativeElement.querySelector('form') as HTMLFormElement).dispatchEvent(
            new Event('submit')
        );
        fixture.componentInstance.onLogin();
        fixture.detectChanges();

        expect(authApi.login$).toHaveBeenCalledTimes(1);

        response$.next('a-token');
        response$.complete();
        fixture.detectChanges();

        expect(authApi.login$).toHaveBeenCalledTimes(1);
        expect(session.start).toHaveBeenCalledTimes(1);
        expect(session.start).toHaveBeenCalledWith('a-token');
        expect(submit().classList).not.toContain('ui-button--loading');
        expect(submit().disabled).toBe(false);
    });

    it('stops waiting after a failure so the user can submit again', () => {
        authApi.login$
            .mockReturnValueOnce(throwError(() => new Error('401')))
            .mockReturnValueOnce(of('a-token'));
        const fixture = setup();
        fixture.componentInstance.form.setValue({
            email: 'a@b.sk',
            password: 'secret',
            hasExtendedSessionLifetime: false
        });
        const submit = () => query('login-submit')?.querySelector('button') as HTMLButtonElement;

        fixture.componentInstance.onLogin();
        fixture.detectChanges();

        expect(error()).not.toBeNull();
        expect(submit().classList).not.toContain('ui-button--loading');
        expect(submit().disabled).toBe(false);

        fixture.componentInstance.onLogin();
        fixture.detectChanges();

        expect(authApi.login$).toHaveBeenCalledTimes(2);
        expect(error()).toBeNull();
        expect(session.start).toHaveBeenCalledTimes(1);
        expect(session.start).toHaveBeenCalledWith('a-token');
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

        expect(toggle.getAttribute('aria-label')).toBe('Show password');

        toggle.click();
        fixture.detectChanges();

        expect(toggle.getAttribute('aria-label')).toBe('Hide password');
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
        expect(query('login-use-password')?.textContent).toContain(
            'Sign in with email and password'
        );

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

        expect(query('login-use-password')?.textContent).toContain(
            'Emergency sign-in with a password'
        );
        expect(query('login-break-glass-hint')).toBeNull();

        click('login-use-password');
        fixture.detectChanges();

        expect(document.querySelector('#password')).not.toBeNull();
        expect(query('login-break-glass-hint')?.textContent).toContain(
            'only break-glass accounts can sign in with a password'
        );
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

        expect(query('login-sso-unavailable')?.textContent).toContain(
            'Company sign-in is temporarily unavailable'
        );
    });

    it('translates the SSO error from the query string', () => {
        withSsoError('account_not_provisioned');
        setup();

        expect(query('login-sso-error')?.textContent).toContain(
            'You have no account here yet. Ask your administrator for access.'
        );
    });
});

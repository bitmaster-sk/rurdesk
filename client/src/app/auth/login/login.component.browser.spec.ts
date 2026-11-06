import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { RouterTestingModule } from '@angular/router/testing';
import { TranslateModule } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { UiModule } from '../../ui/ui.module';
import { AuthRobotStub } from 'src/testing/stubs';
import { AuthApi } from '../api/auth.api.service';
import {
    IconEye,
    IconEyeOff,
    TablerIconComponent,
    provideTablerIcons
} from '@tabler/icons-angular';
import { SessionService } from '../service/session.service';
import { AuthTokenStore } from '../store/auth-token.store';
import { LoginComponent } from './login.component';

describe('LoginComponent (browser)', () => {
    let authApi: { login$: ReturnType<typeof vi.fn> };
    let tokenStore: { hasToken: ReturnType<typeof vi.fn> };
    let session: { start: ReturnType<typeof vi.fn>; end: ReturnType<typeof vi.fn> };

    function error(): HTMLElement | null {
        return document.querySelector('.login__error');
    }

    beforeEach(async () => {
        authApi = { login$: vi.fn() };
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
                provideTablerIcons({ IconEye, IconEyeOff })
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
});

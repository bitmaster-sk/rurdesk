import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { RouterTestingModule } from '@angular/router/testing';
import { TranslateModule } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { UiModule } from '../../ui/ui.module';
import { AuthRobotStub } from 'src/testing/stubs';
import { AuthApi } from '../api/auth.api.service';
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
                AuthRobotStub
            ],
            providers: [
                { provide: AuthApi, useValue: authApi },
                { provide: AuthTokenStore, useValue: tokenStore },
                { provide: SessionService, useValue: session }
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
        expect(error()).toBeNull();

        fixture.componentInstance.onLogin();
        fixture.detectChanges();

        expect(error()).not.toBeNull();
    });

    it('does not show an error message on a successful login', () => {
        authApi.login$.mockReturnValue(of('a-token'));
        const fixture = setup();

        fixture.componentInstance.onLogin();
        fixture.detectChanges();

        expect(error()).toBeNull();
        expect(session.start).toHaveBeenCalledWith('a-token');
    });

    it('clears the error once the user edits the form', () => {
        authApi.login$.mockReturnValue(throwError(() => new Error('401')));
        const fixture = setup();
        fixture.componentInstance.onLogin();
        fixture.detectChanges();
        expect(error()).not.toBeNull();

        fixture.componentInstance.form.get('password')!.setValue('newpass');
        fixture.detectChanges();

        expect(error()).toBeNull();
    });
});

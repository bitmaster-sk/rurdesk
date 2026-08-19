import { HttpErrorResponse } from '@angular/common/http';
import { Injector, runInInjectionContext } from '@angular/core';
import { FormBuilder } from '@angular/forms';
import { Router } from '@angular/router';
import { of, throwError } from 'rxjs';
import { RegisterComponent } from './register.component';
import { AuthApi } from '../api/auth.api.service';
import { SessionService } from '../service/session.service';

const VALID_FORM = {
    name: 'Admin',
    email: 'admin@example.com',
    credentials: { password: 'secret', password2: 'secret' }
};

function httpError(status: number, url: string): HttpErrorResponse {
    return new HttpErrorResponse({ status, url });
}

describe('RegisterComponent', () => {
    let component: RegisterComponent;
    let router: { navigate: ReturnType<typeof vi.fn> };
    let authApi: {
        register$: ReturnType<typeof vi.fn>;
        login$: ReturnType<typeof vi.fn>;
    };
    let session: { start: ReturnType<typeof vi.fn>; end: ReturnType<typeof vi.fn> };

    beforeEach(() => {
        router = { navigate: vi.fn() };
        authApi = {
            register$: vi.fn().mockReturnValue(of(undefined)),
            login$: vi.fn().mockReturnValue(of('token-123'))
        };
        session = { start: vi.fn(), end: vi.fn() };
        const injector = Injector.create({
            providers: [
                { provide: FormBuilder, useValue: new FormBuilder() },
                { provide: Router, useValue: router },
                { provide: AuthApi, useValue: authApi },
                { provide: SessionService, useValue: session }
            ]
        });
        component = runInInjectionContext(injector, () => new RegisterComponent());
        component.ngOnInit();
    });

    it('registers, auto-logs-in, stores the token and navigates home', () => {
        component.form.setValue(VALID_FORM);
        component.onRegister();

        expect(authApi.register$).toHaveBeenCalledWith({
            name: 'Admin',
            email: 'admin@example.com',
            password: 'secret'
        });
        expect(session.start).toHaveBeenCalledWith('token-123');
        expect(component.errorKey()).toBeNull();
    });

    it('shows the closed message when the server returns 403', () => {
        authApi.register$.mockReturnValue(throwError(() => httpError(403, '/api/public/register')));
        component.form.setValue(VALID_FORM);
        component.onRegister();

        expect(component.errorKey()).toBe('REGISTER.CLOSED');
        expect(router.navigate).not.toHaveBeenCalled();
    });

    it('shows a generic message on other server errors', () => {
        authApi.register$.mockReturnValue(throwError(() => httpError(500, '/api/public/register')));
        component.form.setValue(VALID_FORM);
        component.onRegister();

        expect(component.errorKey()).toBe('REGISTER.FAILED');
    });

    it('redirects to /login when registration succeeds but auto-login fails', () => {
        authApi.login$.mockReturnValue(throwError(() => httpError(401, '/api/public/login')));
        component.form.setValue(VALID_FORM);
        component.onRegister();

        expect(router.navigate).toHaveBeenCalledWith(['/login']);
        expect(session.start).not.toHaveBeenCalled();
    });

    it('does not call the server and flags mismatched passwords', () => {
        component.form.setValue({
            ...VALID_FORM,
            credentials: { password: 'secret', password2: 'different' }
        });
        component.onRegister();

        expect(authApi.register$).not.toHaveBeenCalled();
        expect(component.errorKey()).toBe('REGISTER.PASSWORD.MISMATCH');
    });

    it('does not call the server on an incomplete form', () => {
        component.onRegister();

        expect(authApi.register$).not.toHaveBeenCalled();
        expect(component.errorKey()).toBe('REGISTER.INVALID');
    });

    it('clears the error as soon as the user edits the form', () => {
        component.onRegister(); // invalid form → error shown
        expect(component.errorKey()).toBe('REGISTER.INVALID');

        component.form.patchValue({ name: 'A' });
        expect(component.errorKey()).toBeNull();
    });
});

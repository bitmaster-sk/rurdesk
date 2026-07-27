import { HttpErrorResponse } from '@angular/common/http';
import { FormBuilder } from '@angular/forms';
import { Router } from '@angular/router';
import { of, throwError } from 'rxjs';
import { RegisterComponent } from './register.component';
import { SettingsStore } from '../../core/settings/settings.store';
import { UserService } from '../user.service';

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
    let settingsStore: { load: ReturnType<typeof vi.fn> };
    let sUser: {
        register: ReturnType<typeof vi.fn>;
        login: ReturnType<typeof vi.fn>;
        saveAuthLocal: ReturnType<typeof vi.fn>;
    };

    beforeEach(() => {
        router = { navigate: vi.fn() };
        settingsStore = { load: vi.fn() };
        sUser = {
            register: vi.fn().mockReturnValue(of(undefined)),
            login: vi.fn().mockReturnValue(of('token-123')),
            saveAuthLocal: vi.fn()
        };
        component = new RegisterComponent(
            new FormBuilder(),
            router as unknown as Router,
            sUser as unknown as UserService,
            settingsStore as unknown as SettingsStore
        );
        component.ngOnInit();
    });

    it('registers, auto-logs-in, stores the token and navigates home', () => {
        component.form.setValue(VALID_FORM);
        component.onRegister();

        expect(sUser.register).toHaveBeenCalledWith({
            name: 'Admin',
            email: 'admin@example.com',
            password: 'secret'
        });
        expect(sUser.saveAuthLocal).toHaveBeenCalledWith('token-123');
        expect(settingsStore.load).toHaveBeenCalled();
        expect(router.navigate).toHaveBeenCalledWith(['/']);
        expect(component.errorKey()).toBeNull();
    });

    it('shows the closed message when the server returns 403', () => {
        sUser.register.mockReturnValue(throwError(() => httpError(403, '/api/public/register')));
        component.form.setValue(VALID_FORM);
        component.onRegister();

        expect(component.errorKey()).toBe('REGISTER.CLOSED');
        expect(router.navigate).not.toHaveBeenCalled();
    });

    it('shows a generic message on other server errors', () => {
        sUser.register.mockReturnValue(throwError(() => httpError(500, '/api/public/register')));
        component.form.setValue(VALID_FORM);
        component.onRegister();

        expect(component.errorKey()).toBe('REGISTER.FAILED');
    });

    it('redirects to /login when registration succeeds but auto-login fails', () => {
        sUser.login.mockReturnValue(throwError(() => httpError(401, '/api/public/login')));
        component.form.setValue(VALID_FORM);
        component.onRegister();

        expect(router.navigate).toHaveBeenCalledWith(['/login']);
        expect(sUser.saveAuthLocal).not.toHaveBeenCalled();
    });

    it('does not call the server and flags mismatched passwords', () => {
        component.form.setValue({
            ...VALID_FORM,
            credentials: { password: 'secret', password2: 'different' }
        });
        component.onRegister();

        expect(sUser.register).not.toHaveBeenCalled();
        expect(component.errorKey()).toBe('REGISTER.PASSWORD.MISMATCH');
    });

    it('does not call the server on an incomplete form', () => {
        component.onRegister();

        expect(sUser.register).not.toHaveBeenCalled();
        expect(component.errorKey()).toBe('REGISTER.INVALID');
    });

    it('clears the error as soon as the user edits the form', () => {
        component.onRegister(); // invalid form → error shown
        expect(component.errorKey()).toBe('REGISTER.INVALID');

        component.form.patchValue({ name: 'A' });
        expect(component.errorKey()).toBeNull();
    });
});

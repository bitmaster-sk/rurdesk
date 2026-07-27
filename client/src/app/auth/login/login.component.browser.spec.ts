import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { RouterTestingModule } from '@angular/router/testing';
import { TranslateModule } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { UiModule } from '../../ui/ui.module';
import { AuthRobotStub } from 'src/testing/stubs';
import { UserService } from '../user.service';
import { SettingsStore } from '../../core/settings/settings.store';
import { LoginComponent } from './login.component';

describe('LoginComponent (browser)', () => {
    let userService: {
        login: ReturnType<typeof vi.fn>;
        hasAuthLocal: ReturnType<typeof vi.fn>;
        saveAuthLocal: ReturnType<typeof vi.fn>;
    };

    function error(): HTMLElement | null {
        return document.querySelector('.login__error');
    }

    beforeEach(async () => {
        userService = {
            login: vi.fn(),
            hasAuthLocal: vi.fn().mockReturnValue(false),
            saveAuthLocal: vi.fn()
        };

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
                { provide: UserService, useValue: userService },
                // A successful login calls SettingsStore.load(), which otherwise fires a
                // real HTTP GET /api/private/settings and 404s as an unhandled rejection.
                { provide: SettingsStore, useValue: { load: () => {} } }
            ]
        }).compileComponents();
    });

    function setup() {
        const fixture = TestBed.createComponent(LoginComponent);
        fixture.detectChanges();
        return fixture;
    }

    it('shows an error message when login fails', () => {
        userService.login.mockReturnValue(throwError(() => new Error('401')));
        const fixture = setup();
        expect(error()).toBeNull();

        fixture.componentInstance.onLogin();
        fixture.detectChanges();

        expect(error()).not.toBeNull();
    });

    it('does not show an error message on a successful login', () => {
        userService.login.mockReturnValue(of('a-token'));
        const fixture = setup();

        fixture.componentInstance.onLogin();
        fixture.detectChanges();

        expect(error()).toBeNull();
        expect(userService.saveAuthLocal).toHaveBeenCalledWith('a-token');
    });

    it('clears the error once the user edits the form', () => {
        userService.login.mockReturnValue(throwError(() => new Error('401')));
        const fixture = setup();
        fixture.componentInstance.onLogin();
        fixture.detectChanges();
        expect(error()).not.toBeNull();

        fixture.componentInstance.form.get('password')!.setValue('newpass');
        fixture.detectChanges();

        expect(error()).toBeNull();
    });
});

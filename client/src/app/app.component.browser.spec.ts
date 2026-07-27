import { TestBed } from '@angular/core/testing';
import { RouterTestingModule } from '@angular/router/testing';
import { UiToastStub } from 'src/testing/stubs';
import { AppComponent } from './app.component';
import { SettingsStore } from './core/settings/settings.store';
import { UserService } from './auth/user.service';
import { HotkeyService } from './core/command/hotkey.service';

describe('AppComponent', () => {
    beforeEach(async () => {
        // AppComponent's constructor calls SettingsStore.load(), which fires a real
        // HTTP GET /api/private/settings. Stub the store so no request escapes the
        // test (an unmocked request 404s and surfaces as an unhandled rejection).
        await TestBed.configureTestingModule({
            imports: [RouterTestingModule, UiToastStub],
            declarations: [AppComponent],
            providers: [
                { provide: SettingsStore, useValue: { load: () => {} } },
                // AppComponent also injects UserService (→ CookieService) and
                // HotkeyService; stub both so the shell constructs without the full
                // DI graph. hasAuthLocal:false keeps the settings load path off too.
                { provide: UserService, useValue: { hasAuthLocal: () => false } },
                { provide: HotkeyService, useValue: { start: () => {} } }
            ]
        }).compileComponents();
    });

    it('should create the app', () => {
        const fixture = TestBed.createComponent(AppComponent);
        const app = fixture.componentInstance;
        expect(app).toBeTruthy();
    });

    it(`should have as title 'issue-client'`, () => {
        const fixture = TestBed.createComponent(AppComponent);
        const app = fixture.componentInstance;
        expect(app.title).toEqual('issue-client');
    });
});

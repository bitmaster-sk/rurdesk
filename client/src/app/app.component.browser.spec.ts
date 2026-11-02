import { TestBed } from '@angular/core/testing';
import { RouterTestingModule } from '@angular/router/testing';
import { UiToastStub } from 'src/testing/stubs';
import { AppComponent } from './app.component';
import { SettingsStore } from './core/settings/settings.store';
import { LicenseStore } from './core/license/license.store';
import { AuthTokenStore } from './auth/store/auth-token.store';
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
                { provide: LicenseStore, useValue: { load: () => {} } },
                // Complete the token-store contract: AppComponent only calls hasToken(),
                // but the root-scoped NoticeService (via NotificationStore) reads getToken()
                // when opening its WebSocket — a mock without it throws a TypeError that
                // NoticeService logs via console.error into the otherwise clean stderr.
                {
                    provide: AuthTokenStore,
                    useValue: {
                        hasToken: () => false,
                        getToken: () => null,
                        clearToken: () => {},
                        saveToken: () => {}
                    }
                },
                { provide: HotkeyService, useValue: { start: () => {} } }
            ]
        }).compileComponents();
    });

    it('should create the app', () => {
        const fixture = TestBed.createComponent(AppComponent);
        const app = fixture.componentInstance;
        expect(app).toBeTruthy();
    });
});

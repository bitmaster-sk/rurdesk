import { Component, inject } from '@angular/core';
import { AuthTokenStore } from './auth/store/auth-token.store';
import { SettingsStore } from './core/settings/settings.store';
import { HotkeyService } from './core/command/hotkey.service';

@Component({
    selector: 'app-root',
    templateUrl: './app.component.html',
    standalone: false
})
export class AppComponent {
    private readonly settingsStore = inject(SettingsStore);
    private readonly tokenStore = inject(AuthTokenStore);
    private readonly hotkeys = inject(HotkeyService);

    public title = 'issue-client';

    public constructor() {
        // Settings are auth-gated on the server; an anonymous visitor (login/register)
        // runs on the store's fallback defaults and must not trigger a guaranteed 401.
        // After a successful login/registration the auth components load them.
        if (this.tokenStore.hasToken()) {
            this.settingsStore.load();
        }
        // Global keyboard entry point for the command palette (⌘K / bare `/` / `?`).
        // Idempotent and root-provided, so starting here is safe.
        this.hotkeys.start();
    }
}

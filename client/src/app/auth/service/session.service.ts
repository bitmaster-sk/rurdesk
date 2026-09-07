import { Injectable, inject } from '@angular/core';
import { Router } from '@angular/router';
import { SettingsStore } from '../../core/settings/settings.store';
import { NotificationStore } from '../../notification/store/notification.store';
import { AuthApi } from '../api/auth.api.service';
import { AuthTokenStore } from '../store/auth-token.store';

@Injectable({ providedIn: 'root' })
export class SessionService {
    private readonly router = inject(Router);
    private readonly authApi = inject(AuthApi);
    private readonly tokenStore = inject(AuthTokenStore);
    private readonly settingsStore = inject(SettingsStore);
    private readonly notificationStore = inject(NotificationStore);

    public start(token: string): void {
        this.tokenStore.saveToken(token);
        // App bootstrap skips the auth-gated settings load for anonymous visitors;
        // now that we have a token, load them (navigation does not re-bootstrap).
        this.settingsStore.load();
        // Notifications are similarly auth-gated — init the store's bootstrap
        // (idempotent, so safe even if AppComponent already called it).
        this.notificationStore.init();
        void this.router.navigate(['/']);
    }

    public end(redirectTo?: string): void {
        const done = (): void => {
            this.tokenStore.clearToken();
            if (redirectTo !== undefined) {
                void this.router.navigate([redirectTo]);
            }
        };
        this.authApi.logout$().subscribe({ next: done, error: done });
    }
}

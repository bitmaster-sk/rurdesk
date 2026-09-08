import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { AppSettings, UpdateAppSettingsReq } from './model/app-settings.model';

@Injectable({ providedIn: 'root' })
export class SettingsApi {
    private readonly http = inject(HttpClient);

    // readable by any authenticated user
    public load$(): Observable<AppSettings> {
        return this.http.get<AppSettings>('/api/private/settings');
    }

    // admin-only
    public update$(req: UpdateAppSettingsReq): Observable<AppSettings> {
        return this.http.patch<AppSettings>('/api/private/admin/settings', req);
    }
}

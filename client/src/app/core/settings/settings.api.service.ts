import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';

export interface AppSettings {
    tablePageSize: number;
    kanbanPageSize: number;
    ganttBacklogPageSize: number;
    userApiKeyLimit: number;
}

@Injectable({ providedIn: 'root' })
export class SettingsApi {
    private readonly http = inject(HttpClient);

    public getSettings$(): Observable<AppSettings> {
        return this.http.get<AppSettings>('/api/private/settings');
    }
}

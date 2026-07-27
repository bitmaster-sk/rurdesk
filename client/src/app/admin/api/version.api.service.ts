import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { BuildInfo } from '../model/build-info.model';

@Injectable({ providedIn: 'root' })
export class VersionApi {
    private readonly http = inject(HttpClient);

    public getVersion$(): Observable<BuildInfo> {
        // admin-only
        return this.http.get<BuildInfo>('/api/private/admin/version');
    }
}

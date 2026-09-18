import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { LicenseState } from './model/license.model';

@Injectable({ providedIn: 'root' })
export class LicenseApi {
    private readonly http = inject(HttpClient);

    public load$(): Observable<LicenseState> {
        return this.http.get<LicenseState>('/api/private/license');
    }
}

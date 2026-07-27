import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { User } from '../../auth/model/user.model';

@Injectable({ providedIn: 'root' })
export class UserApi {
    private readonly http = inject(HttpClient);

    public loadUsers$(): Observable<User[]> {
        return this.http.get<User[]>('/api/private/users');
    }
}

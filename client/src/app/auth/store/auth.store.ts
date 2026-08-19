import { Injectable, Signal, inject, signal } from '@angular/core';
import { Observable } from 'rxjs';
import { tap } from 'rxjs/operators';
import { AuthApi } from '../api/auth.api.service';
import { User } from '../model/user.model';

@Injectable({ providedIn: 'root' })
export class AuthStore {
    private readonly api = inject(AuthApi);

    private readonly _user = signal<User | null>(null);

    public readonly user: Signal<User | null> = this._user.asReadonly();

    public getUser(): User {
        const user = this._user();
        if (user === null) {
            throw new Error('getUser called before UserResolver loaded the user');
        }
        return user;
    }

    public loadUser$(): Observable<User> {
        return this.api.loadUser$().pipe(tap(u => this._user.set(u)));
    }

    public updateUser$(name: string, colorAvatarBg?: string): Observable<User> {
        return this.api.updateUser$(name, colorAvatarBg).pipe(tap(u => this._user.set(u)));
    }
}

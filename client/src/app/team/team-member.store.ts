import { Injectable, inject } from '@angular/core';
import { BehaviorSubject } from 'rxjs';
import { filter } from 'rxjs/operators';
import { User } from '../auth/model/user.model';
import { TeamMemberApi } from './api/team-member.api.service';

@Injectable({ providedIn: 'root' })
export class TeamMemberStore {
    private readonly api = inject(TeamMemberApi);

    private readonly users = new BehaviorSubject<User[] | null>(null);

    public readonly users$ = this.users.asObservable().pipe(filter((u): u is User[] => u !== null));

    public load(idTeam: number): void {
        this.api.list$(idTeam).subscribe(u => this.users.next(u));
    }
}

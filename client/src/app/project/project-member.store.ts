import { inject, Injectable } from '@angular/core';
import { BehaviorSubject } from 'rxjs';
import { filter, map } from 'rxjs/operators';
import { User } from '../auth/model/user.model';
import { ProjectService } from './project.service';

@Injectable({
    providedIn: 'root'
})
export class ProjectMemberStore {
    private users = new BehaviorSubject<User[] | null>(null);

    public users$ = this.users.asObservable().pipe(filter(users => !!users));

    public usersMap$ = this.users$.pipe(map(users => this.toMap(users)));

    private readonly sProject = inject(ProjectService);

    public load(idProject: number): void {
        this.sProject.loadMembers(idProject).subscribe(users => this.users.next(users));
    }

    private toMap(users: User[]): Map<number, User> {
        const usersMap = new Map<number, User>();
        users.forEach(u => usersMap.set(u.idUser, u));
        return usersMap;
    }
}

import { Injectable, inject } from '@angular/core';
import { BehaviorSubject, Observable } from 'rxjs';
import { filter, map } from 'rxjs/operators';
import { Sprint } from '../model/sprint.model';
import { SprintApi } from '../api/sprint.api.service';

// Pure and unit-testable with an injected `now`. There is no persisted 'active'
// state: among non-closed sprints whose [startAt, endAt) window contains `now`,
// the earliest startAt wins (tie-break: lowest idSprint); otherwise the earliest
// planned sprint; otherwise undefined (chip shows its placeholder). Deterministic
// regardless of the API's start_at DESC ordering.
export function selectCurrentSprint(sprints: Sprint[], now: Date): Sprint | undefined {
    const open = sprints.filter(s => s.state !== 'closed');
    const byStart = (a: Sprint, b: Sprint): number =>
        a.startAt.localeCompare(b.startAt) || a.idSprint - b.idSprint;
    const inWindow = open
        .filter(s => new Date(s.startAt) <= now && now < new Date(s.endAt))
        .sort(byStart);
    if (inWindow.length > 0) {
        return inWindow[0];
    }
    return open.slice().sort(byStart)[0];
}

@Injectable({ providedIn: 'root' })
export class SprintStore {
    private readonly sprintApi = inject(SprintApi);

    private readonly sprints = new BehaviorSubject<Sprint[] | null>(null);

    public readonly sprints$: Observable<Sprint[]> = this.sprints
        .asObservable()
        .pipe(filter((s): s is Sprint[] => !!s));

    public readonly currentSprint$: Observable<Sprint | undefined> = this.sprints$.pipe(
        map(s => selectCurrentSprint(s, new Date()))
    );

    public load(idProject: number): void {
        this.sprintApi.loadByProject$(idProject).subscribe(s => this.sprints.next(s));
    }

    public create(idProject: number, body: Partial<Sprint>): void {
        this.sprintApi.create$(idProject, body).subscribe(() => this.load(idProject));
    }

    public edit(idProject: number, idSprint: number, body: Partial<Sprint>): void {
        this.sprintApi.edit$(idSprint, body).subscribe(() => this.load(idProject));
    }

    public remove(idProject: number, idSprint: number): void {
        this.sprintApi.delete$(idSprint).subscribe(() => this.load(idProject));
    }
}

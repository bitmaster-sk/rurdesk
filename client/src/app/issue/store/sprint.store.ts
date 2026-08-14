import { Injectable, inject } from '@angular/core';
import { BehaviorSubject, combineLatest, Observable, Subject } from 'rxjs';
import { map, startWith, tap } from 'rxjs/operators';
import { utcDayRollover$ } from 'src/app/shared/date/date.util';
import { SprintState } from '../constants/sprint-state.enum';
import { Sprint } from '../model/sprint.model';
import { SprintApi } from '../api/sprint.api.service';

@Injectable({ providedIn: 'root' })
export class SprintStore {
    private readonly sprintApi = inject(SprintApi);

    private selectCurrent(sprints: Sprint[], now: Date): Sprint | undefined {
        const open = sprints.filter(s => s.state !== SprintState.Closed);
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

    private readonly sprints = new BehaviorSubject<Sprint[] | null>(null);

    private readonly loaded = new Subject<Sprint[]>();

    private idLoadedProject: number | null = null;

    public readonly sprints$: Observable<Sprint[]> = this.sprints
        .asObservable()
        .pipe(map(s => s ?? []));

    public readonly currentSprint$: Observable<Sprint | undefined> = combineLatest([
        this.sprints$,
        utcDayRollover$.pipe(startWith(new Date()))
    ]).pipe(map(([sprints, now]) => this.selectCurrent(sprints, now)));

    public readonly currentSprintOnLoad$: Observable<Sprint | undefined> = this.loaded.pipe(
        map(s => this.selectCurrent(s, new Date()))
    );

    public load(idProject: number): void {
        if (this.idLoadedProject !== idProject) {
            this.idLoadedProject = idProject;
            this.sprints.next(null);
        }
        this.sprintApi.loadByProject$(idProject).subscribe({
            next: sprints => this.publish(idProject, sprints),
            error: () => this.publish(idProject, [])
        });
    }

    private publish(idProject: number, sprints: Sprint[]): void {
        if (this.idLoadedProject !== idProject) {
            return;
        }
        this.sprints.next(sprints);
        this.loaded.next(sprints);
    }

    public create(idProject: number, body: Partial<Sprint>): void {
        this.sprintApi.create$(idProject, body).subscribe(() => this.load(idProject));
    }

    public edit(idProject: number, idSprint: number, body: Partial<Sprint>): void {
        this.sprintApi.edit$(idSprint, body).subscribe(() => this.load(idProject));
    }

    public remove$(idProject: number, idSprint: number): Observable<void> {
        return this.sprintApi.delete$(idSprint).pipe(tap(() => this.load(idProject)));
    }
}

import { Injectable } from '@angular/core';
import { BehaviorSubject, Observable } from 'rxjs';
import { filter, map } from 'rxjs/operators';
import { IssueSeverity } from '../model/issue-severity.model';
import { SeverityApi } from '../api/severity.api.service';

@Injectable({
    providedIn: 'root'
})
export class SeverityStore {
    private readonly sSeverity = inject(SeverityApi);

    private severities = new BehaviorSubject<IssueSeverity[] | null>(null);

    public severities$ = this.severities.asObservable().pipe(filter(severities => !!severities));

    public severitiesMap$ = this.severities$.pipe(map(severities => this.toMap(severities)));

    public load(): void {
        this.sSeverity.load$().subscribe(severities => this.severities.next(severities));
    }

    private toMap(severities: IssueSeverity[]): Map<number, IssueSeverity> {
        const severitiesMap = new Map<number, IssueSeverity>();
        severities.forEach(s => severitiesMap.set(s.idSeverity, s));
        return severitiesMap;
    }

    public severitiesByProject$(idProject: number): Observable<IssueSeverity[]> {
        return this.severities$.pipe(
            map(severities => severities.filter(s => s.idProject === idProject))
        );
    }

    public severitiesMapByProject$(idProject: number): Observable<Map<number, IssueSeverity>> {
        return this.severities$.pipe(
            map(severities => severities.filter(s => s.idProject === idProject)),
            map(severities => this.toMap(severities))
        );
    }
}

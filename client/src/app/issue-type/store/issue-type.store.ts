import { Injectable, inject } from '@angular/core';
import { BehaviorSubject, Observable } from 'rxjs';
import { filter, map } from 'rxjs/operators';
import { IssueType } from '../model/issue-type.model';
import { IssueTypeApi } from '../api/issue-type.api.service';

@Injectable({
    providedIn: 'root'
})
export class IssueTypeStore {
    private readonly sIssueType = inject(IssueTypeApi);

    private issueTypes = new BehaviorSubject<IssueType[] | null>(null);

    public issueTypes$ = this.issueTypes.asObservable().pipe(filter(issueTypes => !!issueTypes));

    public issueTypesMap$ = this.issueTypes$.pipe(map(issueTypes => this.toMap(issueTypes)));

    public load(): void {
        this.sIssueType.load$().subscribe(issueTypes => this.issueTypes.next(issueTypes));
    }

    private toMap(issueTypes: IssueType[]): Map<number, IssueType> {
        const issueTypesMap = new Map<number, IssueType>();
        issueTypes.forEach(t => issueTypesMap.set(t.idIssueType, t));
        return issueTypesMap;
    }

    public issueTypesByProject$(idProject: number): Observable<IssueType[]> {
        return this.issueTypes$.pipe(
            map(issueTypes => issueTypes.filter(t => t.idProject === idProject))
        );
    }

    public issueTypesMapByProject$(idProject: number): Observable<Map<number, IssueType>> {
        return this.issueTypes$.pipe(
            map(issueTypes => issueTypes.filter(t => t.idProject === idProject)),
            map(issueTypes => this.toMap(issueTypes))
        );
    }
}

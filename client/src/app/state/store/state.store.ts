import { Injectable } from '@angular/core';
import { BehaviorSubject, Observable } from 'rxjs';
import { filter, map } from 'rxjs/operators';
import { IssueState } from '../model/issue-state.model';
import { StateApi } from '../api/state.api.service';

@Injectable({
    providedIn: 'root'
})
export class StateStore {
    private states = new BehaviorSubject<IssueState[] | null>(null);

    public states$ = this.states.asObservable().pipe(filter(states => !!states));

    constructor(private stateApi: StateApi) {}

    public load(): void {
        this.stateApi.load$().subscribe(states => this.states.next(states));
    }

    private toMap(states: IssueState[]): Map<number, IssueState> {
        const statesMap = new Map<number, IssueState>();
        states.forEach(s => statesMap.set(s.idState, s));
        return statesMap;
    }

    public statesByProject$(idProject: number): Observable<IssueState[]> {
        return this.states$.pipe(map(states => states.filter(s => s.idProject === idProject)));
    }

    public statesMapByProject$(idProject: number): Observable<Map<number, IssueState>> {
        return this.states$.pipe(
            map(states => states.filter(s => s.idProject === idProject)),
            map(states => this.toMap(states))
        );
    }
}

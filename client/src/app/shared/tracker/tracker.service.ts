import { Injectable, inject } from '@angular/core';
import { BehaviorSubject, combineLatest, merge, Observable, ReplaySubject, Subject } from 'rxjs';
import { filter, map, shareReplay, startWith, switchMap, tap } from 'rxjs/operators';
import { TrackFilter } from 'src/app/tracker/entity/track-filter.entity';
import { TrackApi } from './api/track.api.service';
import { TrackerApi } from './api/tracker.api.service';
import { Track, CreateTrackReq, TrackUpdate } from './model/track.model';
import { Tracker } from './model/tracker.model';

@Injectable({
    providedIn: 'root'
})
export class TrackerService {
    private readonly trackerApi = inject(TrackerApi);

    private readonly trackApi = inject(TrackApi);

    private readonly _localTracker$ = new ReplaySubject<Tracker | null>(1);

    private localTracker$ = this._localTracker$.asObservable();

    private _tracksFilter$ = new BehaviorSubject<TrackFilter | null>(null);

    public tracksFilter$ = this._tracksFilter$.asObservable();

    public tracker$ = merge(
        this.localTracker$,
        this.tracksFilter$.pipe(switchMap(() => this.localTracker$))
    ).pipe(shareReplay(1));

    public isTracking$ = this.tracker$.pipe(map(tracker => tracker?.idTracker));

    private tracksChange$ = new Subject<boolean>();

    public tracks$ = combineLatest([
        this._tracksFilter$,
        this.tracksChange$.pipe(startWith(true))
    ]).pipe(
        map(([tracksFilter]) => tracksFilter),
        filter(tracksFilter => tracksFilter != null),
        switchMap(tracksFilter => this.loadTracks$(tracksFilter)),
        shareReplay(1)
    );

    public totalTracked$ = this.tracks$.pipe(
        map(tracks => tracks.reduce((sum, curr) => sum + (curr.tracked ?? 0), 0))
    );

    public loadTracker$(): Observable<Tracker> {
        return this.trackerApi.load$().pipe(tap(tracker => this._localTracker$.next(tracker)));
    }

    public insertTracker$(idProject: number, idIssuePublic: number): Observable<Tracker> {
        return this.trackerApi
            .insert$(idProject, idIssuePublic)
            .pipe(tap(tracker => this._localTracker$.next(tracker)));
    }

    public submitTracker$(idTracker: number): Observable<Track> {
        return this.trackerApi.submit$(idTracker).pipe(
            tap(() => {
                this._localTracker$.next(null);
                this.tracksChange$.next(true);
            })
        );
    }

    public deleteTracker$(idTracker: number): Observable<void> {
        return this.trackerApi.delete$(idTracker).pipe(tap(() => this._localTracker$.next(null)));
    }

    public setTrackFilter(trackFilter: TrackFilter): void {
        this._tracksFilter$.next(trackFilter);
    }

    public loadTracks$(trackFilter: TrackFilter): Observable<Track[]> {
        return this.trackApi.load$(trackFilter);
    }

    public insertTrack$(track: CreateTrackReq): Observable<Track> {
        return this.trackApi.insert$(track).pipe(tap(() => this.tracksChange$.next(true)));
    }

    public updateTrack$(track: TrackUpdate): Observable<Track> {
        return this.trackApi.update$(track).pipe(tap(() => this.tracksChange$.next(true)));
    }

    public deleteTrack$(idTrack: number): Observable<void> {
        return this.trackApi.delete$(idTrack).pipe(tap(() => this.tracksChange$.next(true)));
    }
}

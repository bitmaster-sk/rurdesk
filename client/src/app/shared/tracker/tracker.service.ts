import { Injectable, inject } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { BehaviorSubject, combineLatest, merge, Observable, ReplaySubject, Subject } from 'rxjs';
import { filter, map, shareReplay, startWith, switchMap, tap } from 'rxjs/operators';
import { TrackFilter } from 'src/app/tracker/entity/track-filter.entity';
import { TrackApi } from './api/track.api.service';
import { TrackerApi } from './api/tracker.api.service';
import { TrackerSyncService } from './tracker-sync.service';
import { Track, CreateTrackReq, TrackUpdate } from './model/track.model';
import { Tracker } from './model/tracker.model';

@Injectable({
    providedIn: 'root'
})
export class TrackerService {
    private readonly trackerApi = inject(TrackerApi);

    private readonly trackApi = inject(TrackApi);

    private readonly sync = inject(TrackerSyncService);

    private readonly _localTracker$ = new ReplaySubject<Tracker | null>(1);

    private localTracker$ = this._localTracker$.asObservable();

    private _tracksFilter$ = new BehaviorSubject<TrackFilter | null>(null);

    public tracksFilter$ = this._tracksFilter$.asObservable();

    public tracker$ = merge(
        this.localTracker$,
        this.tracksFilter$.pipe(switchMap(() => this.localTracker$))
    ).pipe(shareReplay(1));

    public isTracking$ = this.tracker$.pipe(map(tracker => tracker?.idTracker));

    public isPaused$ = this.tracker$.pipe(map(tracker => !!tracker?.pausedAt));

    public constructor() {
        this.sync.changed$
            .pipe(
                switchMap(() => this.trackerApi.load$()),
                takeUntilDestroyed()
            )
            .subscribe(tracker => this._localTracker$.next(tracker));
    }

    private tracksChange$ = new Subject<boolean>();

    public tracksChanged$ = this.tracksChange$.asObservable();

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

    public loadTracker$(): Observable<Tracker | null> {
        return this.trackerApi.load$().pipe(tap(tracker => this._localTracker$.next(tracker)));
    }

    public insertTracker$(idProject: number, idIssuePublic: number): Observable<Tracker> {
        return this.trackerApi
            .insert$(idProject, idIssuePublic)
            .pipe(tap(tracker => this.publish(tracker)));
    }

    public submitTracker$(idTracker: number, note?: string | null): Observable<Track> {
        return this.trackerApi.submit$(idTracker, note).pipe(
            tap(() => {
                this.publish(null);
                this.tracksChange$.next(true);
            })
        );
    }

    public pauseTracker$(idTracker: number): Observable<Tracker> {
        return this.trackerApi.pause$(idTracker).pipe(tap(tracker => this.publish(tracker)));
    }

    public resumeTracker$(idTracker: number): Observable<Tracker> {
        return this.trackerApi.resume$(idTracker).pipe(tap(tracker => this.publish(tracker)));
    }

    public deleteTracker$(idTracker: number): Observable<void> {
        return this.trackerApi.delete$(idTracker).pipe(tap(() => this.publish(null)));
    }

    public switchTracker$(
        idProject: number,
        idIssuePublic: number,
        idRunningTracker: number,
        note?: string | null
    ): Observable<Tracker> {
        return this.submitTracker$(idRunningTracker, note).pipe(
            switchMap(() => this.insertTracker$(idProject, idIssuePublic))
        );
    }

    private publish(tracker: Tracker | null): void {
        this._localTracker$.next(tracker);
        this.sync.publish();
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

    public reloadTracks(): void {
        this.tracksChange$.next(true);
    }
}

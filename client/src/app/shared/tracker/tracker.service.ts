import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { BehaviorSubject, combineLatest, merge, Observable, ReplaySubject, Subject } from 'rxjs';
import { filter, map, shareReplay, startWith, switchMap, tap } from 'rxjs/operators';
import { TrackFilter } from 'src/app/tracker/entity/track-filter.entity';
import { Track, TrackInsert, TrackUpdate } from './model/track.model';
import { Tracker } from './model/tracker.model';

@Injectable({
    providedIn: 'root'
})
export class TrackerService {
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
        switchMap(tracksFilter => this.loadTracks(tracksFilter)),
        shareReplay(1)
    );

    public totalTracked$ = this.tracks$.pipe(
        map(tracks => tracks.reduce((sum, curr) => sum + (curr.tracked ?? 0), 0))
    );

    constructor(private http: HttpClient) {}

    public loadTracker(): Observable<Tracker> {
        return this.http.get<Tracker>('/api/private/tracker').pipe(
            map(tracker => this.toTracker(tracker)),
            tap(tracker => this._localTracker$.next(tracker))
        );
    }

    public insertTracker(idProject: number, idIssuePublic: number): Observable<Tracker> {
        return this.http.post<Tracker>(`/api/private/tracker`, { idProject, idIssuePublic }).pipe(
            map(tracker => this.toTracker(tracker)),
            tap(tracker => this._localTracker$.next(tracker))
        );
    }

    public submitTracker(idTracker: number): Observable<Track> {
        return this.http.patch<Track>(`/api/private/tracker/${idTracker}/submit`, {}).pipe(
            map(savedTrack => this.toTrack(savedTrack)),
            tap(() => {
                this._localTracker$.next(null);
                this.tracksChange$.next(true);
            })
        );
    }

    public deleteTracker(idTracker: number): Observable<void> {
        return this.http
            .delete<void>(`/api/private/tracker/${idTracker}`, {})
            .pipe(tap(() => this._localTracker$.next(null)));
    }

    public setTrackFilter(trackFilter: TrackFilter): void {
        this._tracksFilter$.next(trackFilter);
    }

    public loadTracks(trackFilter: TrackFilter): Observable<Track[]> {
        let params = new HttpParams();
        if (trackFilter.idIssue) {
            params = params.set('idIssue', `${trackFilter.idIssue}`);
        }
        if (trackFilter.idProject) {
            params = params.set('idProject', `${trackFilter.idProject}`);
        }
        if (trackFilter.idUser) {
            params = params.set('idUser', `${trackFilter.idUser}`);
        }
        if (trackFilter.from) {
            params = params.set('startFrom', trackFilter.from.toISOString());
        }
        if (trackFilter.to) {
            params = params.set('startTo', trackFilter.to.toISOString());
        }
        return this.http
            .get<Track[]>('/api/private/track', { params })
            .pipe(map(tracks => this.toTracks(tracks)));
    }

    public insertTrack(track: TrackInsert): Observable<Track> {
        return this.http.post<Track>('/api/private/track', track).pipe(
            map(savedTrack => this.toTrack(savedTrack)),
            tap(() => this.tracksChange$.next(true))
        );
    }

    public updateTrack(track: TrackUpdate): Observable<Track> {
        return this.http.patch<Track>(`/api/private/track/${track.idTrack}`, track).pipe(
            map(savedTrack => this.toTrack(savedTrack)),
            tap(() => this.tracksChange$.next(true))
        );
    }

    public deleteTrack(idTrack: number): Observable<void> {
        return this.http
            .delete<void>(`/api/private/track/${idTrack}`)
            .pipe(tap(() => this.tracksChange$.next(true)));
    }

    private toTracks(tracks: Track[]): Track[] {
        return tracks.map(track => this.toTrack(track));
    }

    private toTrack(track: Track): Track {
        track.startAt = track.startAt ? new Date(track.startAt) : null;
        track.endAt = track.endAt ? new Date(track.endAt) : null;
        return track;
    }

    private toTracker(tracker: Tracker): Tracker {
        tracker.startAt = new Date(tracker.startAt);
        return tracker;
    }
}

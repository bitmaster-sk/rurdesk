import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { TrackerConverter } from '../converter/tracker.converter';
import { Track } from '../model/track.model';
import { Tracker } from '../model/tracker.model';

@Injectable({
    providedIn: 'root'
})
export class TrackerApi {
    private readonly http = inject(HttpClient);

    public load$(): Observable<Tracker | null> {
        return this.http
            .get<Tracker>('/api/private/tracker')
            .pipe(
                map(tracker => (tracker?.idTracker ? TrackerConverter.toTracker(tracker) : null))
            );
    }

    public insert$(idProject: number, idIssuePublic: number): Observable<Tracker> {
        return this.http
            .post<Tracker>(`/api/private/tracker`, { idProject, idIssuePublic })
            .pipe(map(tracker => TrackerConverter.toTracker(tracker)));
    }

    public submit$(idTracker: number, note?: string | null): Observable<Track> {
        return this.http
            .patch<Track>(`/api/private/tracker/${idTracker}/submit`, { note: note ?? null })
            .pipe(map(savedTrack => TrackerConverter.toTrack(savedTrack)));
    }

    public pause$(idTracker: number): Observable<Tracker> {
        return this.http
            .patch<Tracker>(`/api/private/tracker/${idTracker}/pause`, {})
            .pipe(map(tracker => TrackerConverter.toTracker(tracker)));
    }

    public resume$(idTracker: number): Observable<Tracker> {
        return this.http
            .patch<Tracker>(`/api/private/tracker/${idTracker}/resume`, {})
            .pipe(map(tracker => TrackerConverter.toTracker(tracker)));
    }

    public delete$(idTracker: number): Observable<void> {
        return this.http.delete<void>(`/api/private/tracker/${idTracker}`, {});
    }
}

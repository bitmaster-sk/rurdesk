import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { TrackFilter } from 'src/app/tracker/entity/track-filter.entity';
import { TrackerConverter } from '../converter/tracker.converter';
import { Track, CreateTrackReq, TrackUpdate } from '../model/track.model';

@Injectable({
    providedIn: 'root'
})
export class TrackApi {
    private readonly http = inject(HttpClient);

    public load$(trackFilter: TrackFilter): Observable<Track[]> {
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
            .pipe(map(tracks => TrackerConverter.toTracks(tracks)));
    }

    public insert$(track: CreateTrackReq): Observable<Track> {
        return this.http
            .post<Track>('/api/private/track', track)
            .pipe(map(savedTrack => TrackerConverter.toTrack(savedTrack)));
    }

    public update$(track: TrackUpdate): Observable<Track> {
        return this.http
            .patch<Track>(`/api/private/track/${track.idTrack}`, track)
            .pipe(map(savedTrack => TrackerConverter.toTrack(savedTrack)));
    }

    public delete$(idTrack: number): Observable<void> {
        return this.http.delete<void>(`/api/private/track/${idTrack}`);
    }
}

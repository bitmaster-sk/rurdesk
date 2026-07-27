import type { HttpClient, HttpParams } from '@angular/common/http';
import { of } from 'rxjs';
import { TrackerService } from './tracker.service';
import { TrackFilter } from 'src/app/tracker/entity/track-filter.entity';
import { Track } from './model/track.model';

describe('TrackerService.loadTracks', () => {
    it('serializes the track filter into query params and maps dates', () => {
        const get = vi.fn().mockReturnValue(of([{ startAt: '2026-01-01T00:00:00Z', endAt: null }]));
        const service = new TrackerService({ get } as unknown as HttpClient);

        let tracks: Track[] = [];
        service
            .loadTracks({
                idIssue: 5,
                idProject: 1,
                from: new Date('2026-01-01T00:00:00Z')
            } as TrackFilter)
            .subscribe(t => (tracks = t));

        const [url, options] = get.mock.calls[0] as [string, { params: HttpParams }];
        expect(url).toBe('/api/private/track');
        expect(options.params.get('idIssue')).toBe('5');
        expect(options.params.get('idProject')).toBe('1');
        expect(options.params.get('startFrom')).toBe('2026-01-01T00:00:00.000Z');
        expect(tracks[0].startAt).toBeInstanceOf(Date);
        expect(tracks[0].endAt).toBeNull();
    });
});

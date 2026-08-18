import { HttpClient, type HttpParams } from '@angular/common/http';
import { Injector, runInInjectionContext } from '@angular/core';
import { of } from 'rxjs';
import { TrackerService } from './tracker.service';
import { Track } from './model/track.model';

describe('TrackerService.loadTracks', () => {
    it('serializes the track filter into query params and maps dates', () => {
        const get = vi.fn().mockReturnValue(of([{ startAt: '2026-01-01T00:00:00Z', endAt: null }]));
        const injector = Injector.create({
            providers: [{ provide: HttpClient, useValue: { get } }]
        });
        const service = runInInjectionContext(injector, () => new TrackerService());

        let tracks: Track[] = [];
        service
            .loadTracks({
                idIssue: 5,
                idProject: 1,
                from: new Date('2026-01-01T00:00:00Z')
            })
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

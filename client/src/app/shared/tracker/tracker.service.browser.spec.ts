import { TestBed } from '@angular/core/testing';
import { Subject, of } from 'rxjs';
import { TrackApi } from './api/track.api.service';
import { TrackerApi } from './api/tracker.api.service';
import { Track } from './model/track.model';
import { Tracker } from './model/tracker.model';
import { TrackerService } from './tracker.service';
import { TrackerSyncService } from './tracker-sync.service';

const makeTracker = (over: Partial<Tracker> = {}): Tracker => {
    const base: Tracker = {
        idTracker: 1,
        idUser: 7,
        idIssue: 10,
        startAt: new Date('2026-09-08T10:00:00Z'),
        pausedAt: null,
        pausedSeconds: 0,
        duration: {},
        idProject: 2,
        idIssuePublic: 142,
        issueTitle: 'Global time tracker',
        projectName: 'RuRdesk'
    };
    return { ...base, ...over };
};

const makeTrack = (): Track => ({
    idTrack: 5,
    idUser: 7,
    idIssue: 10,
    idIssuePublic: 142,
    idProject: 2,
    issueTitle: 'Global time tracker',
    tracked: 3600,
    startAt: new Date('2026-09-08T10:00:00Z'),
    endAt: new Date('2026-09-08T11:00:00Z'),
    note: null
});

describe('TrackerService (browser)', () => {
    let changed$: Subject<void>;
    let publish: ReturnType<typeof vi.fn>;
    let trackerApi: {
        load$: ReturnType<typeof vi.fn>;
        insert$: ReturnType<typeof vi.fn>;
        submit$: ReturnType<typeof vi.fn>;
        pause$: ReturnType<typeof vi.fn>;
        resume$: ReturnType<typeof vi.fn>;
        delete$: ReturnType<typeof vi.fn>;
    };

    beforeEach(() => {
        changed$ = new Subject<void>();
        publish = vi.fn();
        trackerApi = {
            load$: vi.fn(() => of(makeTracker())),
            insert$: vi.fn(() => of(makeTracker({ idTracker: 2, idIssue: 20 }))),
            submit$: vi.fn(() => of(makeTrack())),
            pause$: vi.fn(() => of(makeTracker({ pausedAt: new Date() }))),
            resume$: vi.fn(() => of(makeTracker({ pausedSeconds: 600 }))),
            delete$: vi.fn(() => of(undefined))
        };

        TestBed.configureTestingModule({
            providers: [
                TrackerService,
                { provide: TrackerApi, useValue: trackerApi },
                { provide: TrackApi, useValue: { load$: () => of([]) } },
                { provide: TrackerSyncService, useValue: { changed$, publish } }
            ]
        });
    });

    function currentTracker(service: TrackerService): Tracker | null {
        let seen: Tracker | null = null;
        service.tracker$.subscribe(tracker => (seen = tracker));
        return seen;
    }

    it('tells the other tabs whenever a timer starts', () => {
        const service = TestBed.inject(TrackerService);

        service.insertTracker$(2, 142).subscribe();

        expect(publish).toHaveBeenCalled();
        expect(currentTracker(service)?.idTracker).toBe(2);
    });

    it('clears the timer once it is submitted', () => {
        const service = TestBed.inject(TrackerService);
        service.insertTracker$(2, 142).subscribe();

        service.submitTracker$(2, 'wrote the spec').subscribe();

        expect(trackerApi.submit$).toHaveBeenCalledWith(2, 'wrote the spec');
        expect(currentTracker(service)).toBeNull();
    });

    it('records the pause so the elapsed time stops growing', () => {
        const service = TestBed.inject(TrackerService);

        service.pauseTracker$(1).subscribe();

        expect(currentTracker(service)?.pausedAt).toBeInstanceOf(Date);
    });

    it('resumes into a running timer that remembers the pause', () => {
        const service = TestBed.inject(TrackerService);

        service.resumeTracker$(1).subscribe();

        expect(currentTracker(service)?.pausedAt).toBeNull();
        expect(currentTracker(service)?.pausedSeconds).toBe(600);
    });

    it('submits the running timer before starting the new one', () => {
        const service = TestBed.inject(TrackerService);
        const order: string[] = [];
        trackerApi.submit$.mockImplementation(() => {
            order.push('submit');
            return of(makeTrack());
        });
        trackerApi.insert$.mockImplementation(() => {
            order.push('insert');
            return of(makeTracker({ idTracker: 3 }));
        });

        service.switchTracker$(2, 200, 1).subscribe();

        expect(order).toEqual(['submit', 'insert']);
        expect(currentTracker(service)?.idTracker).toBe(3);
    });

    it('picks up a timer started in another tab', () => {
        const service = TestBed.inject(TrackerService);
        trackerApi.load$.mockReturnValue(of(makeTracker({ idTracker: 99 })));

        changed$.next();

        expect(currentTracker(service)?.idTracker).toBe(99);
    });
});

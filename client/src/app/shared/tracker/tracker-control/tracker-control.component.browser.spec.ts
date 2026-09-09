import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { BehaviorSubject, of } from 'rxjs';
import { User } from 'src/app/auth/model/user.model';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { TrackApi } from '../api/track.api.service';
import { Tracker } from '../model/tracker.model';
import { TrackerService } from '../tracker.service';
import { TrackerControlComponent } from './tracker-control.component';

const ME: User = { idUser: 7, name: 'Me', email: 'm@m.sk', colorAvatarBg: '#111' };

const runningFor = (seconds: number): Tracker => ({
    idTracker: 1,
    idUser: 7,
    idIssue: 10,
    startAt: new Date(Date.now() - seconds * 1000),
    pausedAt: null,
    pausedSeconds: 0,
    duration: {},
    idProject: 2,
    idIssuePublic: 142,
    issueTitle: 'Global time tracker',
    projectName: 'RuRdesk'
});

describe('TrackerControlComponent (browser)', () => {
    let tracker$: BehaviorSubject<Tracker | null>;
    let pauseTracker$: ReturnType<typeof vi.fn>;
    let resumeTracker$: ReturnType<typeof vi.fn>;
    let submitTracker$: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        tracker$ = new BehaviorSubject<Tracker | null>(null);
        pauseTracker$ = vi.fn(() => of(runningFor(60)));
        resumeTracker$ = vi.fn(() => of(runningFor(60)));
        submitTracker$ = vi.fn(() => of({ idTrack: 1 }));

        await TestBed.configureTestingModule({
            declarations: [TrackerControlComponent],
            providers: [
                { provide: AuthStore, useValue: { user: signal(ME) } },
                { provide: TrackApi, useValue: { load$: () => of([]) } },
                {
                    provide: TrackerService,
                    useValue: {
                        tracker$,
                        pauseTracker$,
                        resumeTracker$,
                        submitTracker$,
                        deleteTracker$: () => of(undefined)
                    }
                }
            ]
        })
            .overrideComponent(TrackerControlComponent, { set: { template: '' } })
            .compileComponents();
    });

    function create() {
        const fixture = TestBed.createComponent(TrackerControlComponent);
        fixture.detectChanges();
        return fixture.componentInstance as unknown as {
            tracker: () => Tracker | null;
            elapsedLabel: () => string;
            isPaused: () => boolean;
            isRunaway: () => boolean;
            todayTotalLabel: () => string;
            sparkBars: () => { height: number; isNow: boolean }[];
            onTogglePause: () => void;
            onConfirm: () => void;
        };
    }

    it('shows nothing in the header while no timer runs', () => {
        expect(create().tracker()).toBeNull();
    });

    it('counts the elapsed time as a clock', () => {
        const component = create();
        tracker$.next(runningFor(3671));

        expect(component.elapsedLabel()).toBe('01:01:11');
    });

    it('flags a timer that has been forgotten', () => {
        const component = create();
        tracker$.next(runningFor(9 * 3600));

        expect(component.isRunaway()).toBe(true);
    });

    it('does not flag a paused timer no matter how old', () => {
        const component = create();
        const paused = runningFor(30 * 3600);
        paused.pausedAt = new Date();
        tracker$.next(paused);

        expect(component.isPaused()).toBe(true);
        expect(component.isRunaway()).toBe(false);
    });

    it('pauses a running timer and resumes a paused one', () => {
        const component = create();
        tracker$.next(runningFor(60));
        component.onTogglePause();
        expect(pauseTracker$).toHaveBeenCalledWith(1);

        const paused = runningFor(60);
        paused.pausedAt = new Date();
        tracker$.next(paused);
        component.onTogglePause();
        expect(resumeTracker$).toHaveBeenCalledWith(1);
    });

    it('submits without a note unless the note field was opened', () => {
        const component = create();
        tracker$.next(runningFor(60));

        component.onConfirm();

        expect(submitTracker$).toHaveBeenCalledWith(1, null);
    });

    it('marks the current bucket in the day chart', () => {
        const component = create();
        tracker$.next(runningFor(60));

        const bars = component.sparkBars();
        expect(bars).toHaveLength(10);
        expect(bars.filter(bar => bar.isNow)).toHaveLength(1);
    });
});

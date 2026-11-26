import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { RouterLink, provideRouter } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { BehaviorSubject, of } from 'rxjs';
import { page } from 'vitest/browser';
import { User } from 'src/app/auth/model/user.model';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { UiModule } from 'src/app/ui/ui.module';
import { TablerIconStub } from 'src/testing/stubs';
import { TrackApi } from '../api/track.api.service';
import { Tracker } from '../model/tracker.model';
import { TrackerService } from '../tracker.service';
import { TrackerControlVariant } from '../constants/tracker-control-variant.enum';
import { TrackerControlComponent } from './tracker-control.component';
// The spacing tokens the side menu geometry depends on live in the global styles.
import 'src/styles.scss';

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

@Component({
    standalone: false,
    template: `
        <div style="width: 12rem">
            <app-tracker-control [variant]="TrackerControlVariant.Side" />
        </div>
    `
})
class SideMenuHost {
    protected readonly TrackerControlVariant = TrackerControlVariant;
}

describe('TrackerControlComponent in the side menu (browser)', () => {
    let tracker$: BehaviorSubject<Tracker | null>;

    beforeEach(async () => {
        tracker$ = new BehaviorSubject<Tracker | null>(null);

        await TestBed.configureTestingModule({
            declarations: [TrackerControlComponent, SideMenuHost],
            imports: [
                UiModule,
                ReactiveFormsModule,
                RouterLink,
                TranslateModule.forRoot(),
                TablerIconStub
            ],
            providers: [
                provideRouter([]),
                { provide: AuthStore, useValue: { user: signal(ME) } },
                { provide: TrackApi, useValue: { load$: () => of([]) } },
                {
                    provide: TrackerService,
                    useValue: {
                        tracker$,
                        pauseTracker$: () => of(runningFor(60)),
                        resumeTracker$: () => of(runningFor(60)),
                        submitTracker$: () => of({ idTrack: 1 }),
                        deleteTracker$: () => of(undefined)
                    }
                }
            ]
        }).compileComponents();
    });

    afterEach(async () => {
        await page.viewport(414, 896);
    });

    async function render(): Promise<HTMLElement> {
        const fixture = TestBed.createComponent(SideMenuHost);
        fixture.autoDetectChanges();
        await fixture.whenStable();
        return fixture.nativeElement as HTMLElement;
    }

    const byTestId = (element: HTMLElement, id: string): HTMLElement | null =>
        element.querySelector<HTMLElement>(`[data-testid="${id}"]`);

    it('takes no room while no timer runs', async () => {
        const element = await render();

        expect(byTestId(element, 'tracker-control')).toBeNull();
        expect(element.querySelector('app-tracker-control')!.getBoundingClientRect().height).toBe(
            0
        );
    });

    it('shows the time and pause across the menu row, without the issue link', async () => {
        const element = await render();

        tracker$.next(runningFor(60));

        await vi.waitFor(() =>
            expect(byTestId(element, 'tracker-control-clock')?.textContent?.trim()).toBe('00:01:00')
        );
        expect(byTestId(element, 'tracker-control-pause')).not.toBeNull();
        expect(byTestId(element, 'tracker-control-issue-link')).toBeNull();
        const row = element.firstElementChild!.getBoundingClientRect();
        const control = byTestId(element, 'tracker-control')!.getBoundingClientRect();
        expect(control.width).toBeGreaterThan(row.width * 0.8);
    });

    it('keeps the time and pause whole inside the menu row', async () => {
        const element = await render();

        tracker$.next(runningFor(60));

        await vi.waitFor(() => expect(byTestId(element, 'tracker-control-pause')).not.toBeNull());
        const control = byTestId(element, 'tracker-control')!.getBoundingClientRect();
        const pause = byTestId(element, 'tracker-control-pause')!.getBoundingClientRect();
        for (const id of ['tracker-control-clock', 'tracker-control-pause']) {
            const box = byTestId(element, id)!.getBoundingClientRect();
            expect(box.width, id).toBeGreaterThan(0);
            expect(box.left, id).toBeGreaterThanOrEqual(control.left);
            expect(box.right, id).toBeLessThanOrEqual(control.right + 0.5);
        }
        const digits = element.querySelector('.ctl-digits')!.getBoundingClientRect();
        expect(digits.right).toBeLessThanOrEqual(pause.left);
    });

    it('opens the details beside the menu, not over it', async () => {
        // Desktop width: the details panel is wider than the default phone viewport leaves.
        await page.viewport(1280, 800);
        const element = await render();
        tracker$.next(runningFor(60));
        await vi.waitFor(() => expect(byTestId(element, 'tracker-control-clock')).not.toBeNull());

        byTestId(element, 'tracker-control-clock')!.click();

        // The overlay positions itself after the next render, so wait for the final place.
        const menuRight = element.firstElementChild!.getBoundingClientRect().right;
        await vi.waitFor(() => {
            const pane = document.querySelector<HTMLElement>('.cdk-overlay-pane.tracker-popover');
            expect(pane?.getBoundingClientRect().left).toBeGreaterThanOrEqual(menuRight + 4);
        });
    });
});

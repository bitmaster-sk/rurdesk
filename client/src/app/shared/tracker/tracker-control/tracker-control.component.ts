import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { FormControl } from '@angular/forms';
import { endOfDay, startOfDay } from 'date-fns';
import { BehaviorSubject, of, timer } from 'rxjs';
import { catchError, switchMap } from 'rxjs/operators';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { DurationConverter } from 'src/app/shared/duration/duration.converter';
import { DurationFormatter } from 'src/app/shared/duration/duration.formatter';
import { TrackApi } from '../api/track.api.service';
import { TrackerConverter } from '../converter/tracker.converter';
import { Tracker } from '../model/tracker.model';
import { TrackerService } from '../tracker.service';

const RUNAWAY_SECONDS = 8 * 3600;

const SPARK_BUCKETS = 10;

@Component({
    selector: 'app-tracker-control',
    templateUrl: './tracker-control.component.html',
    styleUrls: ['./tracker-control.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class TrackerControlComponent {
    private readonly trackerService = inject(TrackerService);

    private readonly trackApi = inject(TrackApi);

    private readonly authStore = inject(AuthStore);

    protected readonly tracker = toSignal(this.trackerService.tracker$, { initialValue: null });

    private readonly tick = toSignal(timer(0, 1000), { initialValue: 0 });

    protected readonly elapsedSeconds = computed(() => {
        this.tick();
        const tracker = this.tracker();
        return tracker ? TrackerConverter.toElapsedSeconds(tracker) : 0;
    });

    protected readonly isPaused = computed(() => !!this.tracker()?.pausedAt);

    protected readonly isRunaway = computed(
        () => !this.isPaused() && this.elapsedSeconds() >= RUNAWAY_SECONDS
    );

    protected readonly elapsedLabel = computed(() =>
        DurationFormatter.secondsToClock(this.elapsedSeconds())
    );

    protected readonly noteControl = new FormControl<string>('');

    private readonly todayReload$ = new BehaviorSubject<void>(undefined);

    private readonly todayTracks = toSignal(
        this.todayReload$.pipe(
            switchMap(() => {
                const idUser = this.authStore.user()?.idUser;
                if (!idUser) {
                    return of([]);
                }
                const now = new Date();
                return this.trackApi
                    .load$({
                        idUser,
                        from: startOfDay(now),
                        to: endOfDay(now)
                    })
                    .pipe(catchError(() => of([])));
            })
        ),
        { initialValue: [] }
    );

    protected readonly todayIssueCount = computed(() => {
        const issues = new Set(this.todayTracks().map(track => track.idIssue));
        const tracker = this.tracker();
        if (tracker) {
            issues.add(tracker.idIssue);
        }
        return issues.size;
    });

    protected readonly todayTotalLabel = computed(() => {
        const logged = this.todayTracks().reduce((sum, track) => sum + (track.tracked ?? 0), 0);
        return DurationFormatter.durationToString(
            DurationConverter.secondsToDuration(logged + this.elapsedSeconds())
        );
    });

    /** Ten equal buckets across today, so the popover shows when the work happened. */
    protected readonly sparkBars = computed(() => {
        const now = new Date();
        const dayStart = startOfDay(now).getTime();
        const bucketMs = (endOfDay(now).getTime() - dayStart) / SPARK_BUCKETS;
        const buckets = new Array<number>(SPARK_BUCKETS).fill(0);

        for (const track of this.todayTracks()) {
            if (!track.startAt) {
                continue;
            }
            const index = Math.floor((track.startAt.getTime() - dayStart) / bucketMs);
            if (index >= 0 && index < SPARK_BUCKETS) {
                buckets[index] += track.tracked ?? 0;
            }
        }

        const nowIndex = Math.min(
            SPARK_BUCKETS - 1,
            Math.floor((now.getTime() - dayStart) / bucketMs)
        );
        buckets[nowIndex] += this.elapsedSeconds();

        const peak = Math.max(...buckets, 1);
        return buckets.map((seconds, index) => ({
            height: seconds > 0 ? Math.max(12, Math.round((seconds / peak) * 100)) : 0,
            isNow: index === nowIndex
        }));
    });

    protected onTogglePause(): void {
        const tracker = this.tracker();
        if (!tracker) {
            return;
        }
        const request$ = tracker.pausedAt
            ? this.trackerService.resumeTracker$(tracker.idTracker)
            : this.trackerService.pauseTracker$(tracker.idTracker);
        request$.subscribe();
    }

    protected onConfirm(): void {
        const tracker = this.tracker();
        if (!tracker) {
            return;
        }
        const note = this.noteControl.value?.trim();
        this.trackerService.submitTracker$(tracker.idTracker, note || null).subscribe(() => {
            this.resetNote();
            this.todayReload$.next();
        });
    }

    protected onDiscard(): void {
        const tracker = this.tracker();
        if (!tracker) {
            return;
        }
        this.trackerService.deleteTracker$(tracker.idTracker).subscribe(() => this.resetNote());
    }

    protected onPopoverOpened(): void {
        this.todayReload$.next();
    }

    protected issueLink(tracker: Tracker): unknown[] {
        return ['/project', tracker.idProject, 'issue', tracker.idIssuePublic];
    }

    private resetNote(): void {
        this.noteControl.reset('');
    }
}

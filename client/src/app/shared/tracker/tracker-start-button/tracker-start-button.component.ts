import { ChangeDetectionStrategy, Component, computed, inject, input, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { TrackerService } from '../tracker.service';

@Component({
    selector: 'app-tracker-start-button',
    templateUrl: './tracker-start-button.component.html',
    styleUrls: ['./tracker-start-button.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class TrackerStartButtonComponent {
    private readonly trackerService = inject(TrackerService);

    public readonly idProject = input.required<number>();

    public readonly idIssuePublic = input.required<number>();

    public readonly isIconOnly = input(false);

    public readonly issueTitle = input<string>();

    public readonly projectName = input<string>();

    protected readonly tracker = toSignal(this.trackerService.tracker$, { initialValue: null });

    protected readonly isTrackingThis = computed(() => {
        const tracker = this.tracker();
        return (
            !!tracker &&
            tracker.idProject === this.idProject() &&
            tracker.idIssuePublic === this.idIssuePublic()
        );
    });

    protected readonly runningTracker = computed(() =>
        this.isTrackingThis() ? null : this.tracker()
    );

    protected readonly isPaused = computed(() => !!this.tracker()?.pausedAt);

    protected readonly isSwitchOpen = signal(false);

    protected onStart(): void {
        this.trackerService.insertTracker$(this.idProject(), this.idIssuePublic()).subscribe();
    }

    protected onSwitch(): void {
        const tracker = this.tracker();
        if (!tracker) {
            return;
        }
        this.trackerService
            .switchTracker$(this.idProject(), this.idIssuePublic(), tracker.idTracker)
            .subscribe();
    }

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

    protected onSubmit(): void {
        const tracker = this.tracker();
        if (!tracker) {
            return;
        }
        this.trackerService.submitTracker$(tracker.idTracker).subscribe();
    }
}

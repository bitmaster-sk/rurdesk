import { ChangeDetectionStrategy, Component, computed, input, model, output } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { timer } from 'rxjs';
import { DurationFormatter } from 'src/app/shared/duration/duration.formatter';
import { TrackerConverter } from '../converter/tracker.converter';
import { Tracker } from '../model/tracker.model';

@Component({
    selector: 'app-tracker-switch-dialog',
    templateUrl: './tracker-switch-dialog.component.html',
    styleUrls: ['./tracker-switch-dialog.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class TrackerSwitchDialogComponent {
    public readonly visible = model<boolean>(false);

    public readonly tracker = input.required<Tracker>();

    public readonly idIssuePublic = input.required<number>();

    public readonly issueTitle = input<string>();

    public readonly projectName = input<string>();

    public readonly confirmed = output<void>();

    private readonly tick = toSignal(timer(0, 1000), { initialValue: 0 });

    protected readonly elapsedLabel = computed(() => {
        this.tick();
        return DurationFormatter.secondsToClock(TrackerConverter.toElapsedSeconds(this.tracker()));
    });

    protected onConfirm(): void {
        this.visible.set(false);
        this.confirmed.emit();
    }

    protected onCancel(): void {
        this.visible.set(false);
    }
}

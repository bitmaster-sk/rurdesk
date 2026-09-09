import { ChangeDetectionStrategy, Component, computed, inject, input, signal } from '@angular/core';
import { FormControl, FormGroup, Validators } from '@angular/forms';
import { User } from 'src/app/auth/model/user.model';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { DurationConverter } from 'src/app/shared/duration/duration.converter';
import { DurationFormatter } from 'src/app/shared/duration/duration.formatter';
import { DurationParser } from 'src/app/shared/duration/duration.parser';
import { DurationValidator } from 'src/app/shared/duration/duration.validator';
import { Track } from 'src/app/shared/tracker/model/track.model';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';

@Component({
    selector: 'app-activity-time-item',
    templateUrl: './activity-time-item.component.html',
    styleUrls: ['./activity-time-item.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class ActivityTimeItemComponent {
    private readonly trackerService = inject(TrackerService);

    private readonly authStore = inject(AuthStore);

    public readonly track = input.required<Track>();
    public readonly user = input<User | undefined>(undefined);

    protected readonly isEditing = signal(false);

    protected readonly canEdit = computed(
        () => this.authStore.user()?.idUser === this.track().idUser
    );

    protected readonly form = new FormGroup({
        tracked: new FormControl<string>('', [Validators.required, DurationValidator.duration]),
        note: new FormControl<string>('')
    });

    public formatDuration(seconds: number): string {
        return DurationFormatter.durationToString(DurationConverter.secondsToDuration(seconds));
    }

    protected onStartEdit(): void {
        const track = this.track();
        this.form.setValue({
            tracked: this.formatDuration(track.tracked ?? 0),
            note: track.note ?? ''
        });
        this.isEditing.set(true);
    }

    protected onCancelEdit(): void {
        this.isEditing.set(false);
    }

    protected onSaveEdit(): void {
        if (this.form.invalid) {
            return;
        }
        const track = this.track();
        const tracked = DurationConverter.durationToSeconds(
            DurationParser.stringToDuration(this.form.controls.tracked.value ?? '')
        );
        this.trackerService
            .updateTrack$({
                idTrack: track.idTrack,
                idIssue: track.idIssue,
                tracked,
                endAt: track.endAt,
                note: this.form.controls.note.value?.trim() || null
            })
            .subscribe(() => this.isEditing.set(false));
    }

    protected onDelete(): void {
        this.trackerService.deleteTrack$(this.track().idTrack).subscribe();
    }
}

import { ChangeDetectionStrategy, Component, computed, inject, input, signal } from '@angular/core';
import { User } from 'src/app/auth/model/user.model';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { DurationConverter } from 'src/app/shared/duration/duration.converter';
import { DurationFormatter } from 'src/app/shared/duration/duration.formatter';
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

    public formatDuration(seconds: number): string {
        return DurationFormatter.durationToString(DurationConverter.secondsToDuration(seconds));
    }

    protected onStartEdit(): void {
        this.isEditing.set(true);
    }

    protected onDelete(): void {
        this.trackerService.deleteTrack$(this.track().idTrack).subscribe();
    }
}

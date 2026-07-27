import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { User } from 'src/app/auth/model/user.model';
import { DurationConverter } from 'src/app/shared/duration/duration.converter';
import { DurationFormatter } from 'src/app/shared/duration/duration.formatter';
import { Track } from 'src/app/shared/tracker/model/track.model';

@Component({
    selector: 'app-activity-time-item',
    templateUrl: './activity-time-item.component.html',
    styleUrls: ['./activity-time-item.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class ActivityTimeItemComponent {
    public readonly track = input.required<Track>();
    public readonly user = input<User | undefined>(undefined);

    public formatDuration(seconds: number): string {
        return DurationFormatter.durationToString(DurationConverter.secondsToDuration(seconds));
    }
}

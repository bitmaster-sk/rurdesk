import {
    ChangeDetectionStrategy,
    Component,
    computed,
    inject,
    input,
    OnInit,
    output
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { FormControl, FormGroup, Validators } from '@angular/forms';
import { isSameDay, startOfDay, startOfToday } from 'date-fns';
import { DurationConverter } from 'src/app/shared/duration/duration.converter';
import { DurationFormatter } from 'src/app/shared/duration/duration.formatter';
import { DurationParser } from 'src/app/shared/duration/duration.parser';
import { DurationValidator } from 'src/app/shared/duration/duration.validator';
import { UiDaySelectFormat } from 'src/app/ui/components/day-select/day-select-label';
import { Track } from '../model/track.model';
import { TrackDayValidator } from '../track-day.validator';
import { TrackerService } from '../tracker.service';

let nextFormId = 0;

@Component({
    selector: 'app-track-edit-form',
    templateUrl: './track-edit-form.component.html',
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class TrackEditFormComponent implements OnInit {
    private readonly trackerService = inject(TrackerService);

    public readonly track = input.required<Track>();

    public readonly saved = output<Track>();

    public readonly cancelled = output<void>();

    protected readonly today = startOfToday();

    protected readonly originalDay = computed(() =>
        startOfDay(this.track().startAt ?? this.track().endAt ?? new Date())
    );

    protected readonly UiDaySelectFormat = UiDaySelectFormat;

    private readonly idPrefix = `track-edit-${nextFormId++}`;
    protected readonly dayId = `${this.idPrefix}-day`;
    protected readonly durationId = `${this.idPrefix}-duration`;
    protected readonly noteId = `${this.idPrefix}-note`;

    protected readonly form = new FormGroup({
        tracked: new FormControl<string>('', [Validators.required, DurationValidator.duration]),
        note: new FormControl<string>(''),
        day: new FormControl<Date | null>(null, [
            Validators.required,
            TrackDayValidator.notInFuture
        ])
    });

    private readonly dayChanges = toSignal(this.form.controls.day.valueChanges);

    protected readonly isDayChanged = computed(() => {
        const day = this.dayChanges();
        return !!day && !isSameDay(day, this.originalDay());
    });

    public ngOnInit(): void {
        const track = this.track();
        this.form.setValue({
            tracked: this.formatDuration(track.tracked ?? 0),
            note: track.note ?? '',
            day: this.originalDay()
        });
    }

    public formatDuration(seconds: number): string {
        return DurationFormatter.durationToString(DurationConverter.secondsToDuration(seconds));
    }

    protected onSaveEdit(): void {
        if (this.form.invalid) {
            return;
        }
        const track = this.track();
        const tracked = DurationConverter.durationToSeconds(
            DurationParser.stringToDuration(this.form.controls.tracked.value ?? '')
        );
        const day = this.form.controls.day.value;
        this.trackerService
            .updateTrack$({
                idTrack: track.idTrack,
                idIssue: track.idIssue,
                tracked,
                startAt: day && this.isDayChanged() ? startOfDay(day) : track.startAt,
                note: this.form.controls.note.value?.trim() || null
            })
            .subscribe(saved => this.saved.emit(saved));
    }

    protected onCancelEdit(): void {
        this.cancelled.emit();
    }
}

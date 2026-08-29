import { Component, Input, OnChanges, OnInit, SimpleChanges, inject } from '@angular/core';
import { FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
import { DurationParser } from 'src/app/shared/duration/duration.parser';
import { DurationConverter } from 'src/app/shared/duration/duration.converter';
import { TrackForm } from 'src/app/shared/tracker/model/track.model';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';
import { DurationFormatter } from 'src/app/shared/duration/duration.formatter';

interface TrackEditForm {
    idTrack: FormControl<number | null>;
    idIssue: FormControl<number | null>;
    idUser: FormControl<number | null>;
    tracked: FormControl<string | null>;
    endAt: FormControl<Date | null>;
}

@Component({
    selector: 'app-track-form',
    templateUrl: './track-form.component.html',
    standalone: false
})
export class TrackFormComponent implements OnInit, OnChanges {
    private readonly fb = inject(FormBuilder);
    private readonly sTracker = inject(TrackerService);

    @Input() public track: TrackForm | null = null;

    public form: FormGroup<TrackEditForm> = this.fb.group<TrackEditForm>({
        idTrack: this.fb.control<number | null>(null),
        idIssue: this.fb.control<number | null>(null, [Validators.required]),
        idUser: this.fb.control<number | null>(null, [Validators.required]),
        tracked: this.fb.control<string | null>(null, [Validators.min(0)]),
        endAt: this.fb.control<Date | null>(null, [Validators.required])
    });

    public ngOnInit(): void {
        if (this.track) {
            this.FormValues = this.track;
        }
    }

    public ngOnChanges(changes: SimpleChanges): void {
        if (changes['track'] && !changes['track'].isFirstChange() && this.track) {
            this.FormValues = this.track;
        }
    }

    public onSaveTrack(): void {
        const value = this.form.value;
        const idIssue = value.idIssue ?? 0;
        const tracked = DurationConverter.durationToSeconds(
            DurationParser.stringToDuration(`${value.tracked ?? ''}`)
        );
        const idTrack = value.idTrack ?? 0;
        const saver =
            value.idTrack === null
                ? this.sTracker.insertTrack({ idIssue, tracked, endAt: value.endAt ?? null })
                : this.sTracker.updateTrack({
                      idTrack,
                      idIssue,
                      tracked,
                      endAt: value.endAt ?? null
                  });
        saver.subscribe(() => {
            this.onResetTrack();
        });
    }

    public onResetTrack(): void {
        this.form.reset({
            idTrack: null,
            idUser: this.track?.idUser,
            idIssue: this.track?.idIssue,
            tracked: null,
            endAt: null
        });
    }

    private set FormValues(track: TrackForm) {
        if (!track) {
            this.form.reset();
            return;
        }
        this.form.setValue({
            idTrack: track.idTrack,
            idUser: track.idUser,
            idIssue: track.idIssue,
            tracked: DurationFormatter.durationToString(
                DurationConverter.secondsToDuration(track.tracked ?? 0)
            ),
            endAt: track.endAt
        });
    }
}

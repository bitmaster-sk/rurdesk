import { Component, Input, OnChanges, OnInit, SimpleChanges } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { DurationParser } from 'src/app/shared/duration/duration.parser';
import { DurationConverter } from 'src/app/shared/duration/duration.converter';
import { TrackForm } from 'src/app/shared/tracker/model/track.model';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';
import { DurationFormatter } from 'src/app/shared/duration/duration.formatter';

@Component({
    selector: 'app-track-form',
    templateUrl: './track-form.component.html',
    standalone: false
})
export class TrackFormComponent implements OnInit, OnChanges {
    private readonly fb = inject(FormBuilder);
    private readonly sTracker = inject(TrackerService);

    @Input() public track: TrackForm | null = null;

    public form: FormGroup = this.fb.group({
        idTrack: [null],
        idIssue: [null, [Validators.required]],
        idUser: [null, [Validators.required]],
        tracked: [null, [Validators.min(0)]],
        endAt: [null, [Validators.required]]
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
        const value = this.form.value as {
            idTrack: number | null;
            idIssue: number;
            tracked: string | null;
            endAt: Date | null;
        };
        const tracked = DurationConverter.durationToSeconds(
            DurationParser.stringToDuration(`${value.tracked}`)
        );
        const saver =
            value.idTrack === null
                ? this.sTracker.insertTrack({ idIssue: value.idIssue, tracked, endAt: value.endAt })
                : this.sTracker.updateTrack({
                      idTrack: value.idTrack,
                      idIssue: value.idIssue,
                      tracked,
                      endAt: value.endAt
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

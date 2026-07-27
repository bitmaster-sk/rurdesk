import { Component, Input, OnChanges, OnInit, SimpleChanges } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { DurationParser } from 'src/app/shared/duration/duration.parser';
import { DurationConverter } from 'src/app/shared/duration/duration.converter';
import { Track } from 'src/app/shared/tracker/model/track.model';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';
import { DurationFormatter } from 'src/app/shared/duration/duration.formatter';

@Component({
    selector: 'app-track-form',
    templateUrl: './track-form.component.html',
    standalone: false
})
export class TrackFormComponent implements OnInit, OnChanges {
    @Input() track: Track = null;

    public form: FormGroup = this.fb.group({
        idTrack: [null],
        idIssue: [null, [Validators.required]],
        idUser: [null, [Validators.required]],
        tracked: [null, [Validators.min(0)]],
        endAt: [null, [Validators.required]]
    });

    constructor(
        private fb: FormBuilder,
        private sTracker: TrackerService
    ) {}

    public ngOnInit(): void {
        this.FormValues = this.track;
    }

    public ngOnChanges(changes: SimpleChanges): void {
        if (changes['track'] && !changes['track'].isFirstChange()) {
            this.FormValues = this.track;
        }
    }

    public onSaveTrack(): void {
        const track: Track = this.form.value;
        track.tracked = DurationConverter.durationToSeconds(
            DurationParser.stringToDuration(`${track.tracked}`)
        );
        const saver = !track.idTrack
            ? this.sTracker.insertTrack(track)
            : this.sTracker.updateTrack(track);
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

    private set FormValues(track: Track) {
        if (!track) {
            this.form.reset();
            return;
        }
        this.form.setValue({
            idTrack: track.idTrack,
            idUser: track.idUser,
            idIssue: track.idIssue,
            tracked: DurationFormatter.durationToString(
                DurationConverter.secondsToDuration(track.tracked)
            ),
            endAt: track.endAt
        });
    }
}

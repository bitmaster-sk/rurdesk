import { Component, input, output, computed, inject } from '@angular/core';
import { combineLatest, timer } from 'rxjs';
import { distinctUntilChanged, filter, map, switchMap } from 'rxjs/operators';
import { toSignal } from '@angular/core/rxjs-interop';
import { Issue } from 'src/app/issue/model/issue.model';
import { TrackerService } from './tracker.service';
import { Tracker } from './model/tracker.model';
import { FormControl, Validators } from '@angular/forms';
import { Project } from 'src/app/project/model/project.model';
import { DurationConverter } from '../duration/duration.converter';
import { DurationParser } from '../duration/duration.parser';
import { DurationValidator } from '../duration/duration.validator';
import { IssueService } from 'src/app/issue/issue.service';
import { Track } from './model/track.model';

@Component({
    selector: 'app-tracker',
    templateUrl: './tracker.component.html',
    styleUrls: ['./tracker.component.scss'],
    standalone: false
})
export class TrackerComponent {
    private readonly sTracker = inject(TrackerService);
    private readonly sIssue = inject(IssueService);

    public global = input<boolean>(false);

    public issue = input<Issue | null>(null);

    public project = input<Project | null>(null);

    public inputId = input<string>();

    public trackAdded = output<Track>();

    private tracker = toSignal(this.sTracker.tracker$, { initialValue: null });

    public isTracking = toSignal(this.sTracker.isTracking$, {
        initialValue: undefined
    });

    public showTracker = computed(() => {
        const issue = this.issue();
        const tracker = this.tracker();
        const global = this.global();
        const project = this.project();
        return (
            (!global &&
                !!tracker?.idTracker &&
                !!issue &&
                !!project &&
                issue?.idIssue === tracker?.idIssue) ||
            (global && !!tracker?.idTracker)
        );
    });

    public liveTracker = toSignal(
        combineLatest([this.sTracker.tracker$, timer(0, 1000)]).pipe(
            map(([tracker]) => tracker),
            filter(tracker => tracker !== null),
            map(tracker => ({
                ...tracker,
                duration: DurationConverter.trackerToDuration(tracker)
            }))
        ),
        { initialValue: null }
    );

    public trackerIssue = toSignal(
        this.sTracker.tracker$.pipe(
            distinctUntilChanged((x, y) => x?.idIssue === y?.idIssue),
            filter((tracker): tracker is Tracker => !!tracker?.idTracker),
            switchMap(tracker => this.sIssue.loadIssue(tracker.idProject, tracker.idIssuePublic))
        ),
        { initialValue: null }
    );

    public trackedControl = new FormControl(null, [
        Validators.required,
        DurationValidator.duration
    ]);

    public onSaveTrack(): void {
        const issue = this.issue();
        if (!issue) {
            return;
        }
        const seconds = DurationConverter.durationToSeconds(
            DurationParser.stringToDuration(this.trackedControl.value ?? '')
        );
        this.sTracker.insertTrack({ idIssue: issue.idIssue, tracked: seconds }).subscribe(track => {
            this.trackedControl.reset();
            this.trackAdded.emit(track);
        });
    }

    public onStartTracker(): void {
        const issue = this.issue();
        const project = this.project();
        if (!issue || !project) {
            return;
        }
        this.sTracker.insertTracker(project.idProject, issue.idIssuePublic).subscribe();
    }

    public onSubmitTracker(tracker: Tracker): void {
        this.sTracker
            .submitTracker(tracker.idTracker)
            .subscribe(track => this.trackAdded.emit(track));
    }

    public onStopTracker(tracker: Tracker): void {
        this.sTracker.deleteTracker(tracker.idTracker).subscribe();
    }
}

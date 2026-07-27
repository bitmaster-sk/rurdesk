import { DatePipe } from '@angular/common';
import {
    ChangeDetectionStrategy,
    Component,
    computed,
    effect,
    inject,
    OnInit,
    signal
} from '@angular/core';
import { addDays, subDays } from 'date-fns';
import { combineLatest, merge, Observable, ReplaySubject, Subject } from 'rxjs';
import { first, map, switchMap } from 'rxjs/operators';
import { UserService } from 'src/app/auth/user.service';
import { PinDestinationType } from 'src/app/pin/constant/pin-destination-type.enum';
import { PinView } from 'src/app/pin/entity/pin-view.entity';
import { PinService } from 'src/app/pin/pin.service';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { Track } from 'src/app/shared/tracker/model/track.model';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';

@Component({
    selector: 'app-user',
    templateUrl: './user.page.html',
    styleUrls: ['./user.page.scss'],
    providers: [DatePipe],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class UserPage implements OnInit {
    private readonly sUser = inject(UserService);
    private readonly sPin = inject(PinService);
    private readonly severityStore = inject(SeverityStore);
    private readonly sTracker = inject(TrackerService);

    private readonly _user$ = this.sUser.user$;

    private readonly _loadSignal$ = new ReplaySubject<void>(1);

    private readonly _pinRefreshSignal$ = new Subject<void>();

    public pins$: Observable<PinView[]> = merge(this._loadSignal$, this._pinRefreshSignal$).pipe(
        switchMap(() =>
            combineLatest([this.severityStore.severitiesMap$, this._user$]).pipe(
                first(),
                switchMap(([severities, user]) =>
                    this.sPin.loadPins(user.idUser, PinDestinationType.USER).pipe(
                        map(pins =>
                            pins.map(p => {
                                const sev =
                                    p.issue?.idSeverity != null
                                        ? severities.get(p.issue.idSeverity)
                                        : undefined;
                                return {
                                    idPin: p.idPin,
                                    idSeverity: sev?.idSeverity,
                                    severityColor: sev?.color,
                                    severityName: sev?.title,
                                    idProject: p.issue?.idProject,
                                    idIssuePublic: p.issue?.idIssuePublic,
                                    title: p.issue?.title,
                                    stateName: p.issue?.stateName,
                                    stateIsStart: p.issue?.stateIsStart,
                                    stateIsFinal: p.issue?.stateIsFinal,
                                    assignedToName: p.issue?.assignedToName,
                                    assignedToColorAvatarBg: p.issue?.assignedToColorAvatarBg
                                };
                            })
                        )
                    )
                )
            )
        )
    );

    public tracks$: Observable<Track[]> = this.sTracker.tracks$;

    public weekOffset = signal<number>(0);

    public weekRange = computed(() => {
        const offset = this.weekOffset();
        const to = addDays(new Date(), offset * 7);
        to.setHours(23, 59, 59);
        const from = subDays(to, 6);
        from.setHours(0, 0, 0);
        return { from, to };
    });

    private updateTrackerOnWeekChange = effect(() => {
        // Trigger when weekRange changes (reads the computed)
        const range = this.weekRange();
        // Update filter with current user and week range
        this._user$.pipe(first()).subscribe(user => {
            this.sTracker.setTrackFilter({ idUser: user.idUser, ...range });
        });
    });

    public ngOnInit(): void {
        this._loadSignal$.next();
    }

    public onDeletePin(pin: PinView): void {
        this.sPin.deletePin(pin.idPin).subscribe(() => this._pinRefreshSignal$.next());
    }

    public previousWeek(): void {
        this.weekOffset.update(offset => offset - 1);
    }

    public nextWeek(): void {
        this.weekOffset.update(offset => offset + 1);
    }
}

import { ChangeDetectionStrategy, Component, OnInit, inject, signal } from '@angular/core';
import { toObservable } from '@angular/core/rxjs-interop';
import { combineLatest, merge, Subject } from 'rxjs';
import { map } from 'rxjs/operators';
import { User } from 'src/app/auth/model/user.model';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { Track, TrackForm } from 'src/app/shared/tracker/model/track.model';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';
import { UserApi } from 'src/app/user/api/user.api.service';

@Component({
    selector: 'app-track-table',
    templateUrl: './track-table.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class TrackTableComponent implements OnInit {
    private readonly trackerService = inject(TrackerService);
    private readonly userApi = inject(UserApi);
    private readonly authStore = inject(AuthStore);

    public users = signal<Map<number, User>>(new Map());

    public tracks$ = this.trackerService.tracks$;

    public total$ = this.trackerService.totalTracked$;

    public _track$ = new Subject<TrackForm>();

    public track$ = merge(
        this._track$,
        combineLatest([toObservable(this.authStore.user), this.trackerService.tracksFilter$]).pipe(
            map(([user, filter]) => {
                if (!user || !filter) {
                    return null;
                }
                if (filter.idIssue == null) {
                    return null;
                }
                const seed: TrackForm = {
                    idTrack: null,
                    idIssue: filter.idIssue,
                    idUser: user.idUser,
                    tracked: null,
                    endAt: null,
                    note: null
                };
                return seed;
            })
        )
    );

    public ngOnInit(): void {
        this.userApi
            .load$()
            .pipe(
                map(users => {
                    const result = new Map<number, User>();
                    users.forEach(u => result.set(u.idUser, u));
                    return result;
                })
            )
            .subscribe(users => this.users.set(users));
    }

    public onEditTrack(track: Track): void {
        this._track$.next({
            idTrack: track.idTrack,
            idUser: track.idUser,
            idIssue: track.idIssue,
            tracked: track.tracked,
            endAt: track.endAt,
            note: track.note
        });
    }

    public onConfirmDeleteTrack(track: Track): void {
        this.trackerService.deleteTrack$(track.idTrack).subscribe();
    }
}

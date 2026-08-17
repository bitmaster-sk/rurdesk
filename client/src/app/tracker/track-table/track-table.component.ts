import { Component, Input, OnInit } from '@angular/core';
import { combineLatest, merge, Subject } from 'rxjs';
import { map } from 'rxjs/operators';
import { User } from 'src/app/auth/model/user.model';
import { UserService } from 'src/app/auth/user.service';
import { Track, TrackForm } from 'src/app/shared/tracker/model/track.model';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';
import { UserApi } from 'src/app/user/api/user.api.service';

@Component({
    selector: 'app-track-table',
    templateUrl: './track-table.component.html',
    standalone: false
})
export class TrackTableComponent implements OnInit {
    private readonly sTracker = inject(TrackerService);
    private readonly userApi = inject(UserApi);
    private readonly sUser = inject(UserService);

    public users = new Map<number, User>();

    public tracks$ = this.sTracker.tracks$;

    public total$ = this.sTracker.totalTracked$;

    public _track$ = new Subject<TrackForm>();

    public track$ = merge(
        this._track$,
        combineLatest([this.sUser.user$, this.sTracker.tracksFilter$]).pipe(
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
                    endAt: null
                };
                return seed;
            })
        )
    );

    public ngOnInit(): void {
        this.userApi
            .loadUsers$()
            .pipe(
                map(users => {
                    const result = new Map<number, User>();
                    users.forEach(u => result.set(u.idUser, u));
                    return result;
                })
            )
            .subscribe(users => (this.users = users));
    }

    public onEditTrack(track: Track): void {
        this._track$.next({
            idTrack: track.idTrack,
            idUser: track.idUser,
            idIssue: track.idIssue,
            tracked: track.tracked,
            endAt: track.endAt
        });
    }

    public onConfirmDeleteTrack(track: Track): void {
        this.sTracker.deleteTrack(track.idTrack).subscribe();
    }
}

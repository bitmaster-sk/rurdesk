import { Component, Input, OnInit } from '@angular/core';
import { combineLatest, merge, Subject } from 'rxjs';
import { map } from 'rxjs/operators';
import { User } from 'src/app/auth/model/user.model';
import { UserService } from 'src/app/auth/user.service';
import { Track } from 'src/app/shared/tracker/model/track.model';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';
import { UserApi } from 'src/app/user/api/user.api.service';

@Component({
    selector: 'app-track-table',
    templateUrl: './track-table.component.html',
    standalone: false
})
export class TrackTableComponent implements OnInit {
    public users = new Map<number, User>();

    public tracks$ = this.sTracker.tracks$;

    public total$ = this.sTracker.totalTracked$;

    public _track$ = new Subject<Track>();

    public track$ = merge(
        this._track$,
        combineLatest([this.sUser.user$, this.sTracker.tracksFilter$]).pipe(
            map(([user, filter]) => {
                if (!user || !filter) {
                    return null;
                }
                return {
                    idTrack: null,
                    idIssue: filter.idIssue,
                    idUser: user.idUser,
                    tracked: null,
                    endAt: null
                } as Track;
            })
        )
    );

    constructor(
        private sTracker: TrackerService,
        private userApi: UserApi,
        private sUser: UserService
    ) {}

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
        if (track === null) {
            return;
        }
        this._track$.next(track);
    }

    public onConfirmDeleteTrack(track: Track): void {
        this.sTracker.deleteTrack(track.idTrack).subscribe();
    }
}

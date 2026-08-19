import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { BehaviorSubject, of } from 'rxjs';
import { User } from 'src/app/auth/model/user.model';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { TrackForm } from 'src/app/shared/tracker/model/track.model';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';
import { UserApi } from 'src/app/user/api/user.api.service';
import { TrackFilter } from '../entity/track-filter.entity';
import { TrackTableComponent } from './track-table.component';

const ME: User = { idUser: 7, name: 'Me', email: 'm@m.sk', colorAvatarBg: '#111' };

describe('TrackTableComponent (browser)', () => {
    let tracksFilter$: BehaviorSubject<TrackFilter | null>;

    beforeEach(async () => {
        tracksFilter$ = new BehaviorSubject<TrackFilter | null>(null);
        await TestBed.configureTestingModule({
            declarations: [TrackTableComponent],
            providers: [
                { provide: AuthStore, useValue: { user: signal(ME) } },
                {
                    provide: TrackerService,
                    useValue: {
                        tracks$: of([]),
                        totalTracked$: of(0),
                        tracksFilter$,
                        deleteTrack: () => of(undefined)
                    }
                },
                { provide: UserApi, useValue: { loadUsers$: () => of([ME]) } }
            ]
        })
            .overrideComponent(TrackTableComponent, { set: { template: '' } })
            .compileComponents();
    });

    async function lastSeed(): Promise<TrackForm | null | undefined> {
        const fixture = TestBed.createComponent(TrackTableComponent);
        const seen: (TrackForm | null)[] = [];
        fixture.componentInstance.track$.subscribe(t => seen.push(t));
        fixture.detectChanges();
        await fixture.whenStable();
        return seen.at(-1);
    }

    it('seeds a track for the current user when the filter carries an issue', async () => {
        tracksFilter$.next({ idIssue: 42 });

        expect(await lastSeed()).toEqual({
            idTrack: null,
            idIssue: 42,
            idUser: ME.idUser,
            tracked: null,
            endAt: null
        });
    });

    it('seeds nothing when the filter carries no issue', async () => {
        tracksFilter$.next({ idProject: 3 });

        expect(await lastSeed()).toBeNull();
    });
});

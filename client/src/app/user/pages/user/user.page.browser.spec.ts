import { WritableSignal, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { User } from 'src/app/auth/model/user.model';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { PinDestinationType } from 'src/app/pin/constant/pin-destination-type.enum';
import { PinService } from 'src/app/pin/pin.service';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';
import { UserPage } from './user.page';

const ME: User = { idUser: 7, name: 'Me', email: 'm@m.sk', colorAvatarBg: '#111' };

describe('UserPage (browser)', () => {
    let user: WritableSignal<User | null>;
    let loadPins: ReturnType<typeof vi.fn>;
    let setTrackFilter: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        user = signal<User | null>(ME);
        loadPins = vi.fn().mockReturnValue(of([]));
        setTrackFilter = vi.fn();
        await TestBed.configureTestingModule({
            declarations: [UserPage],
            providers: [
                { provide: AuthStore, useValue: { user } },
                { provide: PinService, useValue: { loadPins, deletePin: () => of(undefined) } },
                { provide: SeverityStore, useValue: { severitiesMap$: of(new Map()) } },
                { provide: TrackerService, useValue: { tracks$: of([]), setTrackFilter } }
            ]
        })
            .overrideComponent(UserPage, { set: { template: '' } })
            .compileComponents();
    });

    async function render() {
        const fixture = TestBed.createComponent(UserPage);
        fixture.componentInstance.pins$.subscribe();
        fixture.detectChanges();
        await fixture.whenStable();
        return fixture;
    }

    it('loads the pins of the current user', async () => {
        await render();

        expect(loadPins).toHaveBeenCalledWith(ME.idUser, PinDestinationType.USER);
    });

    it('filters the tracker by the current user when the week changes', async () => {
        const fixture = await render();
        setTrackFilter.mockClear();

        fixture.componentInstance.previousWeek();
        fixture.detectChanges();
        await fixture.whenStable();

        expect(setTrackFilter).toHaveBeenCalledTimes(1);
        expect(setTrackFilter.mock.calls[0][0].idUser).toBe(ME.idUser);
    });

    it('does not refilter the tracker when the profile changes', async () => {
        const fixture = await render();
        setTrackFilter.mockClear();

        user.set({ ...ME, name: 'Renamed' });
        fixture.detectChanges();
        await fixture.whenStable();

        expect(setTrackFilter).not.toHaveBeenCalled();
    });
});

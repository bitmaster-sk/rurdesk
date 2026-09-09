import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { User } from 'src/app/auth/model/user.model';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { Track } from 'src/app/shared/tracker/model/track.model';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';
import { ActivityTimeItemComponent } from './activity-time-item.component';

const ME: User = { idUser: 7, name: 'Me', email: 'm@m.sk', colorAvatarBg: '#111' };

const track = (over: Partial<Track> = {}): Track => {
    const base: Track = {
        idTrack: 3,
        idUser: 7,
        idIssue: 10,
        idIssuePublic: 142,
        idProject: 2,
        issueTitle: 'Global time tracker',
        tracked: 5400,
        startAt: new Date('2026-09-08T10:00:00Z'),
        endAt: new Date('2026-09-08T11:30:00Z'),
        note: null
    };
    return { ...base, ...over };
};

describe('ActivityTimeItemComponent (browser)', () => {
    let updateTrack$: ReturnType<typeof vi.fn>;
    let deleteTrack$: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        updateTrack$ = vi.fn(() => of(track()));
        deleteTrack$ = vi.fn(() => of(undefined));

        await TestBed.configureTestingModule({
            declarations: [ActivityTimeItemComponent],
            providers: [
                { provide: AuthStore, useValue: { user: signal(ME) } },
                { provide: TrackerService, useValue: { updateTrack$, deleteTrack$ } }
            ]
        })
            .overrideComponent(ActivityTimeItemComponent, { set: { template: '' } })
            .compileComponents();
    });

    function create(input: Track) {
        const fixture = TestBed.createComponent(ActivityTimeItemComponent);
        fixture.componentRef.setInput('track', input);
        fixture.detectChanges();
        return fixture.componentInstance as unknown as {
            canEdit: () => boolean;
            isEditing: () => boolean;
            form: {
                controls: {
                    tracked: { value: string; setValue: (value: string) => void };
                    note: { value: string; setValue: (value: string) => void };
                };
            };
            onStartEdit: () => void;
            onCancelEdit: () => void;
            onSaveEdit: () => void;
            onDelete: () => void;
        };
    }

    it('offers editing only on entries the signed-in user logged', () => {
        expect(create(track()).canEdit()).toBe(true);
        expect(create(track({ idUser: 99 })).canEdit()).toBe(false);
    });

    it('opens the form filled with the logged duration and note', () => {
        const component = create(track({ note: 'reviewed the spec' }));

        component.onStartEdit();

        expect(component.isEditing()).toBe(true);
        expect(component.form.controls.tracked.value).toBe('1h 30m');
        expect(component.form.controls.note.value).toBe('reviewed the spec');
    });

    it('saves the edited duration back in seconds', () => {
        const component = create(track());
        component.onStartEdit();
        component.form.controls.tracked.setValue('2h 15m');
        component.form.controls.note.setValue('  wrote the migration  ');

        component.onSaveEdit();

        expect(updateTrack$).toHaveBeenCalledWith(
            expect.objectContaining({
                idTrack: 3,
                tracked: 8100,
                note: 'wrote the migration'
            })
        );
        expect(component.isEditing()).toBe(false);
    });

    it('stores an emptied note as no note at all', () => {
        const component = create(track({ note: 'old' }));
        component.onStartEdit();
        component.form.controls.note.setValue('   ');

        component.onSaveEdit();

        expect(updateTrack$).toHaveBeenCalledWith(expect.objectContaining({ note: null }));
    });

    it('refuses to save an unparseable duration', () => {
        const component = create(track());
        component.onStartEdit();
        component.form.controls.tracked.setValue('');

        component.onSaveEdit();

        expect(updateTrack$).not.toHaveBeenCalled();
    });

    it('leaves the entry untouched when the edit is cancelled', () => {
        const component = create(track());
        component.onStartEdit();

        component.onCancelEdit();

        expect(component.isEditing()).toBe(false);
        expect(updateTrack$).not.toHaveBeenCalled();
    });

    it('deletes the entry', () => {
        create(track()).onDelete();

        expect(deleteTrack$).toHaveBeenCalledWith(3);
    });
});

import { CommonModule } from '@angular/common';
import { Directive, input, output, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { By } from '@angular/platform-browser';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { addDays, startOfDay, startOfToday, subDays } from 'date-fns';
import { of } from 'rxjs';
import en from 'src/assets/i18n/en.json';
import { User } from 'src/app/auth/model/user.model';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { Track } from 'src/app/shared/tracker/model/track.model';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';
import { AvatarStub, TablerIconStub, UiButtonStub, UiDaySelectStub } from 'src/testing/stubs';
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

@Directive({ selector: '[uiConfirm]', standalone: true })
class UiConfirmStub {
    public readonly confirmText = input<string>('');
    public readonly confirmed = output<void>();
}

interface EditInternals {
    form: {
        controls: {
            day: { value: Date | null; setValue: (value: Date) => void };
        };
    };
    onSaveEdit(): void;
}

const formatDay = (value: Date): string =>
    new Intl.DateTimeFormat('en-US', {
        weekday: 'short',
        month: 'short',
        day: 'numeric'
    }).format(value);

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

describe('ActivityTimeItemComponent day editing (browser)', () => {
    let updateTrack$: ReturnType<typeof vi.fn>;
    let fixture: ComponentFixture<ActivityTimeItemComponent>;

    beforeEach(async () => {
        updateTrack$ = vi.fn(() => of(track()));

        await TestBed.configureTestingModule({
            declarations: [ActivityTimeItemComponent],
            imports: [
                CommonModule,
                ReactiveFormsModule,
                TranslateModule.forRoot(),
                TablerIconStub,
                UiButtonStub,
                UiDaySelectStub,
                AvatarStub,
                UiConfirmStub
            ],
            providers: [
                { provide: AuthStore, useValue: { user: signal(ME) } },
                {
                    provide: TrackerService,
                    useValue: { updateTrack$, deleteTrack$: vi.fn(() => of(undefined)) }
                }
            ]
        }).compileComponents();

        const translate = TestBed.inject(TranslateService);
        translate.setTranslation('en', en);
        translate.use('en');
    });

    function openEdit() {
        fixture = TestBed.createComponent(ActivityTimeItemComponent);
        fixture.componentRef.setInput('track', track());
        fixture.detectChanges();
        const editButton = (fixture.nativeElement as HTMLElement).querySelector(
            '[data-testid="track-edit"]'
        ) as HTMLElement;
        editButton.click();
        fixture.detectChanges();
        return {
            component: fixture.componentInstance as unknown as EditInternals,
            el: fixture.nativeElement as HTMLElement
        };
    }

    function saveButton(): UiButtonStub {
        const node = fixture.debugElement.query(By.css('[data-testid="track-edit-save"]'));
        return node.injector.get(UiButtonStub);
    }

    it('opens the form on the day of the entry with the day select and no moves hint', () => {
        const page = openEdit();

        expect(page.component.form.controls.day.value).toEqual(
            startOfDay(new Date('2026-09-08T10:00:00Z'))
        );
        const daySelect = fixture.debugElement
            .query(By.css('[data-testid="track-edit-day"]'))
            .injector.get(UiDaySelectStub);
        expect(daySelect.maxDate()).toEqual(startOfToday());
        expect(page.el.querySelector('[data-testid="track-edit-moves"]')).toBeNull();
    });

    it('keeps the original start time when only the duration changes', () => {
        const page = openEdit();
        const duration = page.el.querySelector(
            '[data-testid="track-edit-duration"]'
        ) as HTMLInputElement;
        duration.value = '2h 15m';
        duration.dispatchEvent(new Event('input'));

        page.component.onSaveEdit();

        expect(updateTrack$).toHaveBeenCalledWith({
            idTrack: 3,
            idIssue: 10,
            tracked: 8100,
            startAt: new Date('2026-09-08T10:00:00Z'),
            note: null
        });
    });

    it('moves the entry to the picked day at midnight and shows the moves hint', () => {
        const page = openEdit();
        const movedTo = subDays(startOfToday(), 2);

        page.component.form.controls.day.setValue(movedTo);
        fixture.detectChanges();

        const hint = page.el.querySelector<HTMLElement>('[data-testid="track-edit-moves"]');
        expect(hint).not.toBeNull();
        expect(hint?.textContent?.trim()).toBe(
            `Moves this entry from ${formatDay(startOfDay(new Date('2026-09-08T10:00:00Z')))} to ${formatDay(movedTo)}.`
        );

        page.component.onSaveEdit();

        expect(updateTrack$).toHaveBeenCalledWith({
            idTrack: 3,
            idIssue: 10,
            tracked: 5400,
            startAt: movedTo,
            note: null
        });
    });

    it('disables saving while the picked day is in the future', () => {
        const page = openEdit();
        expect(saveButton().disabled()).toBe(false);

        page.component.form.controls.day.setValue(addDays(startOfToday(), 1));
        fixture.detectChanges();
        expect(saveButton().disabled()).toBe(true);

        page.component.form.controls.day.setValue(subDays(startOfToday(), 1));
        fixture.detectChanges();
        expect(saveButton().disabled()).toBe(false);
    });
});

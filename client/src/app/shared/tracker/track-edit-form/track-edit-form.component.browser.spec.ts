import { CommonModule } from '@angular/common';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { By } from '@angular/platform-browser';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { addDays, startOfDay, startOfToday, subDays } from 'date-fns';
import { of } from 'rxjs';
import en from 'src/assets/i18n/en.json';
import { UiButtonStub, UiDaySelectStub } from 'src/testing/stubs';
import { Track } from '../model/track.model';
import { TrackerService } from '../tracker.service';
import { TrackEditFormComponent } from './track-edit-form.component';

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

interface EditInternals {
    form: {
        controls: {
            tracked: { value: string; setValue: (value: string) => void };
            note: { value: string; setValue: (value: string) => void };
            day: { value: Date | null; setValue: (value: Date) => void };
        };
    };
    onSaveEdit(): void;
    onCancelEdit(): void;
}

const formatDay = (value: Date): string =>
    new Intl.DateTimeFormat('en-US', {
        weekday: 'short',
        month: 'short',
        day: 'numeric'
    }).format(value);

describe('TrackEditFormComponent (browser)', () => {
    let updateTrack$: ReturnType<typeof vi.fn>;
    let fixture: ComponentFixture<TrackEditFormComponent>;

    beforeEach(async () => {
        updateTrack$ = vi.fn(() => of(track()));

        await TestBed.configureTestingModule({
            declarations: [TrackEditFormComponent],
            imports: [
                CommonModule,
                ReactiveFormsModule,
                TranslateModule.forRoot(),
                UiButtonStub,
                UiDaySelectStub
            ],
            providers: [
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

    function create(input: Track) {
        fixture = TestBed.createComponent(TrackEditFormComponent);
        fixture.componentRef.setInput('track', input);
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

    it('opens filled with the logged duration and note on the day of the entry', () => {
        const page = create(track({ note: 'reviewed the spec' }));

        expect(page.component.form.controls.tracked.value).toBe('1h 30m');
        expect(page.component.form.controls.note.value).toBe('reviewed the spec');
        expect(page.component.form.controls.day.value).toEqual(
            startOfDay(new Date('2026-09-08T10:00:00Z'))
        );
    });

    it('caps the day select at today and shows no moves hint', () => {
        const page = create(track());

        const daySelect = fixture.debugElement
            .query(By.css('[data-testid="track-edit-day"]'))
            .injector.get(UiDaySelectStub);
        expect(daySelect.maxDate()).toEqual(startOfToday());
        expect(page.el.querySelector('[data-testid="track-edit-moves"]')).toBeNull();
    });

    it('saves the edited duration back in seconds', () => {
        const page = create(track());
        page.component.form.controls.tracked.setValue('2h 15m');
        page.component.form.controls.note.setValue('  wrote the migration  ');

        page.component.onSaveEdit();

        expect(updateTrack$).toHaveBeenCalledWith(
            expect.objectContaining({
                idTrack: 3,
                tracked: 8100,
                note: 'wrote the migration'
            })
        );
    });

    it('stores an emptied note as no note at all', () => {
        const page = create(track({ note: 'old' }));
        page.component.form.controls.note.setValue('   ');

        page.component.onSaveEdit();

        expect(updateTrack$).toHaveBeenCalledWith(expect.objectContaining({ note: null }));
    });

    it('refuses to save an unparseable duration', () => {
        const page = create(track());
        page.component.form.controls.tracked.setValue('');

        page.component.onSaveEdit();

        expect(updateTrack$).not.toHaveBeenCalled();
    });

    it('keeps the original start time when only the duration changes', () => {
        const page = create(track());
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
        const page = create(track());
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
        const page = create(track());
        expect(saveButton().disabled()).toBe(false);

        page.component.form.controls.day.setValue(addDays(startOfToday(), 1));
        fixture.detectChanges();
        expect(saveButton().disabled()).toBe(true);

        page.component.form.controls.day.setValue(subDays(startOfToday(), 1));
        fixture.detectChanges();
        expect(saveButton().disabled()).toBe(false);
    });

    it('emits the updated track after saving', () => {
        const updated = track({ tracked: 8100 });
        updateTrack$.mockReturnValue(of(updated));
        const saved = vi.fn();
        const page = create(track());
        fixture.componentInstance.saved.subscribe(saved);

        page.component.onSaveEdit();

        expect(saved).toHaveBeenCalledWith(updated);
    });

    it('emits cancelled and leaves the entry untouched', () => {
        const cancelled = vi.fn();
        const page = create(track());
        fixture.componentInstance.cancelled.subscribe(cancelled);

        page.component.onCancelEdit();

        expect(cancelled).toHaveBeenCalled();
        expect(updateTrack$).not.toHaveBeenCalled();
    });
});

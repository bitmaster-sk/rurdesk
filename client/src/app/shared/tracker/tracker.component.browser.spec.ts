import { CommonModule } from '@angular/common';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { By } from '@angular/platform-browser';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { addDays, startOfDay, startOfToday, subDays } from 'date-fns';
import { BehaviorSubject, of } from 'rxjs';
import en from 'src/assets/i18n/en.json';
import { Issue } from 'src/app/issue/model/issue.model';
import { IssueApi } from 'src/app/issue/api/issue.api.service';
import { Project } from 'src/app/project/model/project.model';
import { TablerIconStub, UiButtonStub, UiDaySelectStub, UiTooltipStub } from 'src/testing/stubs';
import { DurationPipe } from '../duration/duration.pipe';
import { Track } from './model/track.model';
import { TrackerComponent } from './tracker.component';
import { TrackerService } from './tracker.service';

const ISSUE: Issue = {
    idIssue: 10,
    idIssuePublic: 142,
    idProject: 2,
    idState: null,
    idSeverity: null,
    idIssueType: null,
    title: 'Some task',
    description: '',
    tracked: 0
};

const PROJECT: Project = { idProject: 2, name: 'RuRdesk', color: '#111' };

const savedTrack = (): Track => ({
    idTrack: 7,
    idUser: 7,
    idIssue: 10,
    idIssuePublic: 142,
    idProject: 2,
    issueTitle: 'Some task',
    tracked: 3600,
    startAt: startOfToday(),
    endAt: null,
    note: null
});

interface TrackerInternals {
    trackedControl: FormControl<string | null>;
    dayControl: FormControl<Date>;
    onSaveTrack(): void;
}

describe('TrackerComponent (browser)', () => {
    let insertTrack$: ReturnType<typeof vi.fn>;
    let fixture: ComponentFixture<TrackerComponent>;

    beforeEach(async () => {
        insertTrack$ = vi.fn(() => of(savedTrack()));

        await TestBed.configureTestingModule({
            declarations: [TrackerComponent, DurationPipe],
            imports: [
                CommonModule,
                ReactiveFormsModule,
                TranslateModule.forRoot(),
                TablerIconStub,
                UiButtonStub,
                UiDaySelectStub,
                UiTooltipStub
            ],
            providers: [
                { provide: IssueApi, useValue: { loadOne$: () => of(null) } },
                {
                    provide: TrackerService,
                    useValue: {
                        tracker$: new BehaviorSubject(null),
                        isTracking$: of(false),
                        insertTrack$,
                        insertTracker$: () => of(undefined),
                        switchTracker$: () => of(undefined),
                        submitTracker$: () => of(undefined),
                        deleteTracker$: () => of(undefined)
                    }
                }
            ]
        }).compileComponents();

        const translate = TestBed.inject(TranslateService);
        translate.setTranslation('en', en);
        translate.use('en');
    });

    function create() {
        fixture = TestBed.createComponent(TrackerComponent);
        fixture.componentRef.setInput('issue', ISSUE);
        fixture.componentRef.setInput('project', PROJECT);
        fixture.detectChanges();
        const component = fixture.componentInstance as unknown as TrackerInternals;
        const el = fixture.nativeElement as HTMLElement;
        return {
            fixture,
            component,
            hint: () => el.querySelector<HTMLElement>('[data-testid="track-day-hint"]'),
            daySelect: () => el.querySelector<HTMLElement>('[data-testid="track-day"]'),
            saveButton: () =>
                fixture.debugElement
                    .queryAll(By.directive(UiButtonStub))
                    .map(button => button.injector.get(UiButtonStub))[0]
        };
    }

    it('logs to today by default and shows no hint', () => {
        const page = create();

        expect(page.daySelect()).not.toBeNull();
        expect(page.hint()).toBeNull();

        page.component.trackedControl.setValue('1h 30m');
        page.component.onSaveTrack();

        expect(insertTrack$).toHaveBeenCalledWith({
            idIssue: 10,
            tracked: 5400,
            startAt: startOfToday()
        });
    });

    it('shows the hint and logs to the chosen past day', () => {
        const page = create();
        const yesterday = startOfDay(subDays(new Date(), 1));

        page.component.dayControl.setValue(yesterday);
        page.fixture.detectChanges();

        expect(page.hint()).not.toBeNull();
        expect(page.hint()!.textContent).toContain('Logs to');

        page.component.trackedControl.setValue('1h');
        page.component.onSaveTrack();

        expect(insertTrack$).toHaveBeenCalledWith({
            idIssue: 10,
            tracked: 3600,
            startAt: yesterday
        });
    });

    it('resets the day to today after saving', () => {
        const page = create();
        page.component.dayControl.setValue(startOfDay(subDays(new Date(), 1)));
        page.component.trackedControl.setValue('1h');
        page.component.onSaveTrack();

        expect(page.component.dayControl.value).toEqual(startOfToday());

        page.fixture.detectChanges();
        expect(page.hint()).toBeNull();
    });

    it('disables saving while the day is in the future', () => {
        const page = create();
        page.component.trackedControl.setValue('1h');
        page.fixture.detectChanges();
        expect(page.saveButton().disabled()).toBe(false);

        page.component.dayControl.setValue(addDays(startOfToday(), 1));
        page.fixture.detectChanges();
        expect(page.saveButton().disabled()).toBe(true);
    });
});

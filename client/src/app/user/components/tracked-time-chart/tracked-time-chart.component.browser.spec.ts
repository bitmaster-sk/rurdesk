import { CommonModule } from '@angular/common';
import { Component, Directive, input, output } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { of } from 'rxjs';
import en from 'src/assets/i18n/en.json';
import { Track } from 'src/app/shared/tracker/model/track.model';
import { TrackedTimeChartDataset } from 'src/app/user/entity/tracked-time-chart-dataset.entity';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';
import { TablerIconStub, UiDialogStub } from 'src/testing/stubs';
import { beforeEach, describe, expect, it } from 'vitest';
import { TrackedTimeChartComponent } from './tracked-time-chart.component';

describe('TrackedTimeChartComponent (browser)', () => {
    let fixture: ComponentFixture<TrackedTimeChartComponent>;

    const track = (over: Partial<Track>): Track => ({
        idTrack: 1,
        idUser: 1,
        idIssue: 11,
        idIssuePublic: 204,
        idProject: 1,
        issueTitle: 'Trackovanie',
        tracked: 3600,
        startAt: new Date('2026-10-19T09:00:00'),
        endAt: null,
        note: null,
        ...over
    });

    const datasets = (): TrackedTimeChartDataset[] =>
        fixture.componentInstance.chartData().datasets as TrackedTimeChartDataset[];

    beforeEach(async () => {
        await TestBed.configureTestingModule({ declarations: [TrackedTimeChartComponent] })
            .overrideComponent(TrackedTimeChartComponent, { set: { template: '' } })
            .compileComponents();
        fixture = TestBed.createComponent(TrackedTimeChartComponent);
        fixture.componentRef.setInput('tracks', []);
        fixture.componentRef.setInput('from', new Date('2026-10-19T00:00:00'));
        fixture.detectChanges();
    });

    it('renders days as horizontal rows', () => {
        expect(fixture.componentInstance.chartOptions().indexAxis).toBe('y');
    });

    it('maps every tracked issue to its own dataset with hours on the tracked day', () => {
        fixture.componentRef.setInput('tracks', [
            track({
                idIssue: 12,
                idIssuePublic: 211,
                issueTitle: 'Klik na segment',
                tracked: 7200,
                startAt: new Date('2026-10-20T09:00:00')
            }),
            track({ tracked: 3600, startAt: new Date('2026-10-21T09:00:00') })
        ]);
        fixture.detectChanges();

        const all = datasets();
        expect(all.length).toBe(2);

        const ds204 = all.find(ds => ds.idIssuePublic === 204)!;
        expect(ds204.data).toEqual([0, 0, 1, 0, 0, 0, 0]);
        expect(ds204.issueTitle).toBe('Trackovanie');

        const ds211 = all.find(ds => ds.idIssuePublic === 211)!;
        expect(ds211.data).toEqual([0, 2, 0, 0, 0, 0, 0]);
    });

    it('counts a track started at 00:30 into that day', () => {
        fixture.componentRef.setInput('tracks', [
            track({ tracked: 1800, startAt: new Date('2026-10-22T00:30:00') })
        ]);
        fixture.detectChanges();

        expect(datasets()[0].data).toEqual([0, 0, 0, 0.5, 0, 0, 0]);
    });
});

@Directive({ selector: 'canvas[baseChart]', standalone: true })
class BaseChartStub {
    public readonly type = input<string>('bar');
    public readonly data = input<unknown>(null);
    public readonly options = input<unknown>(null);
    public readonly plugins = input<unknown[]>([]);
    public readonly chartClick = output<{ active?: object[] }>();
}

@Directive({ selector: '[uiConfirm]', standalone: true })
class UiConfirmStub {
    public readonly confirmText = input<string>('');
    public readonly confirmed = output<void>();
}

@Component({ selector: 'app-track-edit-form', template: '', standalone: true })
class TrackEditFormStub {
    public readonly track = input<Track | null>(null);
    public readonly saved = output<Track>();
    public readonly cancelled = output<void>();
}

describe('TrackedTimeChartComponent segment click (browser)', () => {
    let fixture: ComponentFixture<TrackedTimeChartComponent>;

    let deleteTrack$: ReturnType<typeof vi.fn>;

    const track = (over: Partial<Track> = {}): Track => ({
        idTrack: 1,
        idUser: 1,
        idIssue: 11,
        idIssuePublic: 204,
        idProject: 1,
        issueTitle: 'Trackovanie',
        tracked: 3600,
        startAt: new Date('2026-10-21T09:00:00'),
        endAt: new Date('2026-10-21T10:00:00'),
        note: null,
        ...over
    });

    const segmentApi = (): {
        selection: () => { idIssue: number; dayKey: string } | null;
        dialogHeader: () => string;
        isDialogOpen: () => boolean;
        onChartClick: (event: { active?: object[] }) => void;
    } =>
        fixture.componentInstance as unknown as {
            selection: () => { idIssue: number; dayKey: string } | null;
            dialogHeader: () => string;
            isDialogOpen: () => boolean;
            onChartClick: (event: { active?: object[] }) => void;
        };

    const openSegment = (): void => {
        segmentApi().onChartClick({ active: [{ datasetIndex: 0, index: 2 }] });
        fixture.detectChanges();
    };

    beforeEach(async () => {
        deleteTrack$ = vi.fn(() => of(undefined));

        await TestBed.configureTestingModule({
            declarations: [TrackedTimeChartComponent],
            imports: [
                CommonModule,
                TranslateModule.forRoot(),
                UiDialogStub,
                TablerIconStub,
                BaseChartStub,
                UiConfirmStub,
                TrackEditFormStub
            ],
            providers: [{ provide: TrackerService, useValue: { deleteTrack$ } }]
        }).compileComponents();

        const translate = TestBed.inject(TranslateService);
        translate.setTranslation('en', en);
        translate.use('en');

        fixture = TestBed.createComponent(TrackedTimeChartComponent);
        fixture.componentRef.setInput('tracks', []);
        fixture.componentRef.setInput('from', new Date('2026-10-19T00:00:00'));
        fixture.detectChanges();
    });

    it('maps the clicked segment to its issue and day and opens the dialog', () => {
        fixture.componentRef.setInput('tracks', [
            track({ note: 'Dokončenie grafu' }),
            track({
                idTrack: 2,
                startAt: new Date('2026-10-21T15:00:00'),
                endAt: new Date('2026-10-21T16:00:00')
            })
        ]);
        fixture.detectChanges();

        const chart = segmentApi();
        chart.onChartClick({ active: [{ datasetIndex: 0, index: 2 }] });
        fixture.detectChanges();

        expect(chart.selection()).toEqual({ idIssue: 11, dayKey: 'Oct 21, 2026' });
        expect(chart.dialogHeader()).toBe('#204 Trackovanie · Oct 21, 2026');
        expect(chart.isDialogOpen()).toBe(true);

        const rows = fixture.nativeElement.querySelectorAll('.chart-track');
        expect(rows.length).toBe(2);
        expect(rows[0].textContent).toContain('9:00');
        expect(rows[0].textContent).toContain('10:00');
        expect(rows[0].textContent).toContain('1h');
        expect(rows[0].textContent).toContain('Dokončenie grafu');
    });

    it('renders one row per track of the issue for the clicked day only', () => {
        fixture.componentRef.setInput('tracks', [
            track(),
            track({
                idTrack: 2,
                startAt: new Date('2026-10-21T15:00:00'),
                endAt: new Date('2026-10-21T16:00:00')
            }),
            track({
                idTrack: 3,
                startAt: new Date('2026-10-22T09:00:00'),
                endAt: new Date('2026-10-22T10:00:00')
            }),
            track({
                idTrack: 4,
                idIssue: 12,
                idIssuePublic: 211,
                startAt: new Date('2026-10-21T09:00:00'),
                endAt: new Date('2026-10-21T10:00:00')
            })
        ]);
        fixture.detectChanges();

        openSegment();

        const rows = fixture.nativeElement.querySelectorAll('.chart-track');
        expect(rows.length).toBe(2);
    });

    it('opens nothing when the click hits no segment', () => {
        fixture.componentRef.setInput('tracks', [track()]);
        fixture.detectChanges();

        segmentApi().onChartClick({ active: [] });
        fixture.detectChanges();

        const chart = segmentApi();
        expect(chart.selection()).toBeNull();
        expect(chart.isDialogOpen()).toBe(false);
        expect(fixture.nativeElement.querySelectorAll('.chart-track').length).toBe(0);
    });

    it('closes the dialog when the selected track disappears from the input', () => {
        fixture.componentRef.setInput('tracks', [track()]);
        fixture.detectChanges();

        openSegment();
        expect(segmentApi().isDialogOpen()).toBe(true);

        fixture.componentRef.setInput('tracks', []);
        fixture.detectChanges();

        expect(segmentApi().isDialogOpen()).toBe(false);
    });

    it('deletes the single track opened directly in the edit form after the confirm is accepted', () => {
        fixture.componentRef.setInput('tracks', [track({ idTrack: 5 })]);
        fixture.detectChanges();

        openSegment();

        (
            fixture.nativeElement.querySelector('[data-testid="chart-track-delete"]') as HTMLElement
        ).click();
        fixture.detectChanges();

        fixture.debugElement
            .query(By.directive(UiConfirmStub))
            .injector.get(UiConfirmStub)
            .confirmed.emit();

        expect(deleteTrack$).toHaveBeenCalledWith(5);
    });

    it('opens the shared edit form directly when the segment has a single track', () => {
        fixture.componentRef.setInput('tracks', [track({ idTrack: 7 })]);
        fixture.detectChanges();

        openSegment();

        const form = fixture.debugElement
            .query(By.directive(TrackEditFormStub))
            .injector.get(TrackEditFormStub);
        expect(form.track()!.idTrack).toBe(7);

        expect(fixture.nativeElement.querySelector('app-track-edit-form')).not.toBeNull();
        expect(fixture.nativeElement.querySelector('[data-testid="chart-track-edit"]')).toBeNull();
        expect(
            fixture.nativeElement.querySelector('[data-testid="chart-track-delete"]')
        ).not.toBeNull();
    });

    it('closes the dialog when the single track edit form is saved', () => {
        fixture.componentRef.setInput('tracks', [track()]);
        fixture.detectChanges();

        openSegment();

        fixture.debugElement
            .query(By.directive(TrackEditFormStub))
            .injector.get(TrackEditFormStub)
            .saved.emit(track());
        fixture.detectChanges();

        expect(segmentApi().isDialogOpen()).toBe(false);
        expect(fixture.nativeElement.querySelector('app-track-edit-form')).toBeNull();
    });

    it('closes the dialog when the single track edit form is cancelled', () => {
        fixture.componentRef.setInput('tracks', [track()]);
        fixture.detectChanges();

        openSegment();

        fixture.debugElement
            .query(By.directive(TrackEditFormStub))
            .injector.get(TrackEditFormStub)
            .cancelled.emit();
        fixture.detectChanges();

        expect(segmentApi().isDialogOpen()).toBe(false);
        expect(fixture.nativeElement.querySelector('app-track-edit-form')).toBeNull();
    });

    it('swaps only the edited row for the shared edit form and back after saving', () => {
        fixture.componentRef.setInput('tracks', [
            track(),
            track({
                idTrack: 2,
                startAt: new Date('2026-10-21T15:00:00'),
                endAt: new Date('2026-10-21T16:00:00')
            })
        ]);
        fixture.detectChanges();

        openSegment();

        (
            fixture.nativeElement.querySelector('[data-testid="chart-track-edit"]') as HTMLElement
        ).click();
        fixture.detectChanges();

        const form = fixture.debugElement
            .query(By.directive(TrackEditFormStub))
            .injector.get(TrackEditFormStub);
        expect(form.track()!.idTrack).toBe(1);

        expect(fixture.nativeElement.querySelectorAll('.chart-track').length).toBe(1);
        expect(
            fixture.nativeElement.querySelectorAll('[data-testid="chart-track-delete"]').length
        ).toBe(1);

        form.saved.emit(track());
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelector('app-track-edit-form')).toBeNull();
        expect(fixture.nativeElement.querySelectorAll('.chart-track').length).toBe(2);
    });
});

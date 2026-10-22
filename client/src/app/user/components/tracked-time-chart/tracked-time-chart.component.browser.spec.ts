import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Track } from 'src/app/shared/tracker/model/track.model';
import { TrackedTimeChartDataset } from 'src/app/user/entity/tracked-time-chart-dataset.entity';
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

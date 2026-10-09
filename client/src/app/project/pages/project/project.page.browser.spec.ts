import { beforeEach, describe, expect, it, vi } from 'vitest';
import { Component, input } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { RouterModule } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { of, Subject } from 'rxjs';
import { ProjectStore } from '../../project.store';
import { ProjectStatStore } from '../../project-stat.store';
import { ProjectMemberStore } from '../../project-member.store';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { PinApi } from 'src/app/pin/api/pin.api.service';
import { ProjectPage } from './project.page';
import { Pin } from 'src/app/pin/model/pin.model';
import { PinDestinationType } from 'src/app/pin/constant/pin-destination-type.enum';
import { PinView } from 'src/app/pin/entity/pin-view.entity';
import { StatsChartEntry } from '../../components/project-stats-chart/project-stats-chart.component';
import { WorkloadEntry } from '../../components/workload-bar-list/workload-bar-list.component';
import { TablerIconStub } from 'src/testing/stubs';

@Component({
    selector: 'app-pin-view',
    standalone: false,
    template: '<span class="pin-title" [textContent]="pin().title"></span>'
})
class PinViewStubComponent {
    public readonly pin = input.required<PinView>();
}

@Component({
    selector: 'app-project-stats-chart',
    standalone: false,
    template: '<span class="chart-label" [textContent]="label()"></span>'
})
class ProjectStatsChartStubComponent {
    public readonly label = input.required<string>();
    public readonly entries = input.required<StatsChartEntry[]>();
    public readonly colors = input<string[]>();
}

@Component({
    selector: 'app-stats-bar-chart',
    standalone: false,
    template: '<span class="bar-chart">bar</span>'
})
class StatsBarChartStubComponent {
    public readonly entries = input.required<StatsChartEntry[]>();
}

@Component({
    selector: 'app-workload-bar-list',
    standalone: false,
    template: '<span class="workload" [textContent]="entries().length"></span>'
})
class WorkloadBarListStubComponent {
    public readonly entries = input.required<WorkloadEntry[]>();
}

describe('ProjectPage (browser)', () => {
    const project = { idProject: 1, name: 'Demo', color: '#123' };
    const pin: Pin = {
        idPin: 1,
        idIssue: 10,
        idPinDestination: 1,
        idPinDestinationType: PinDestinationType.PROJECT,
        issue: {
            idProject: 1,
            idIssuePublic: 1,
            idSeverity: null,
            title: 'Pinned task'
        }
    };
    const pins$ = new Subject<Pin[]>();

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [TranslateModule.forRoot(), RouterModule.forRoot([]), TablerIconStub],
            declarations: [
                ProjectPage,
                PinViewStubComponent,
                ProjectStatsChartStubComponent,
                StatsBarChartStubComponent,
                WorkloadBarListStubComponent
            ],
            providers: [
                {
                    provide: ProjectStore,
                    useValue: { project$: of(project) }
                },
                {
                    provide: ProjectStatStore,
                    useValue: {
                        totalEstimatedSeconds$: of(3600),
                        totalTrackedSeconds$: of(1800),
                        issuesByState$: of([]),
                        issuesBySeverity$: of([]),
                        openIssuesByAssignee$: of([])
                    }
                },
                { provide: ProjectMemberStore, useValue: { usersMap$: of(new Map()) } },
                {
                    provide: SeverityStore,
                    useValue: {
                        severitiesMapByProject$: () => of(new Map())
                    }
                },
                {
                    provide: PinApi,
                    useValue: {
                        load$: vi.fn().mockReturnValue(pins$),
                        delete$: () => of(undefined)
                    }
                }
            ]
        }).compileComponents();
    });

    it('renders project overview after the async project stream emits', async () => {
        const fixture = TestBed.createComponent(ProjectPage);
        fixture.detectChanges();
        await fixture.whenStable();

        expect(fixture.nativeElement.textContent).toContain('HOME.OVERVIEW');
        expect(fixture.nativeElement.textContent).toContain('PROJECT.AI_TOOLS');
    });

    it('renders the pin title after pins$ emits a pin', async () => {
        const fixture = TestBed.createComponent(ProjectPage);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        pins$.next([pin]);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        const title = fixture.nativeElement.querySelector('.pin-title');
        expect(title).toBeTruthy();
        expect(title.textContent).toBe('Pinned task');
    });
});

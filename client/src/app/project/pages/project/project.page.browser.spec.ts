import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { of } from 'rxjs';
import { ProjectStore } from '../../project.store';
import { ProjectStatStore } from '../../project-stat.store';
import { ProjectMemberStore } from '../../project-member.store';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { PinApi } from 'src/app/pin/api/pin.api.service';
import { ProjectPage } from './project.page';

describe('ProjectPage (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [TranslateModule.forRoot()],
            declarations: [ProjectPage],
            providers: [
                {
                    provide: ProjectStore,
                    useValue: { project$: of({ idProject: 1, name: 'Demo', color: '#123' }) }
                },
                {
                    provide: ProjectStatStore,
                    useValue: {
                        totalEstimatedSeconds$: of(0),
                        totalTrackedSeconds$: of(0),
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
                    useValue: { load$: () => of([]), delete$: () => of(undefined) }
                }
            ]
        })
            .overrideComponent(ProjectPage, { set: { template: '' } })
            .compileComponents();
    });

    it('renders after the async project stream emits', async () => {
        const fixture = TestBed.createComponent(ProjectPage);
        fixture.detectChanges();
        await fixture.whenStable();
        expect(fixture.componentInstance).toBeTruthy();
    });
});

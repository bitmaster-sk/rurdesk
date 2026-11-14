import { describe, it, expect, beforeEach } from 'vitest';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { Component, input, output } from '@angular/core';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { of, Subject } from 'rxjs';
import { TranslateModule } from '@ngx-translate/core';

import { IssueDetailPage } from './issue-detail.page';
import { IssueApi } from '../../api/issue.api.service';
import { ProjectStore } from 'src/app/project/project.store';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { CommandPaletteService } from 'src/app/core/command/command-palette.service';
import { AgentRunStore } from 'src/app/agent/store/agent-run.store';
import { LinkScope } from 'src/app/link/service/link-scope.service';
import { UiModule } from 'src/app/ui/ui.module';
import { SplitDialogStub } from 'src/testing/stubs';
import { SplitDialogHostComponent } from '../split-dialog/split-dialog-host.component';
import { Issue } from '../../model/issue.model';
import { Track } from 'src/app/shared/tracker/model/track.model';
import { Fixtures } from 'src/testing/fixtures';

@Component({ selector: 'app-issue-info', template: '', standalone: true })
class IssueInfoStub {
    public readonly issue = input<Issue | null>(null);
    public readonly project = input<unknown>(null);
    public readonly agentRun = input<unknown>(null);
    public readonly splitRequested = output<Issue>();
    public readonly trackAdded = output<Track>();
    public readonly deleteRequested = output<Issue>();
    public readonly cancelAgentRun = output<void>();
    public readonly continueAgentRun = output<void>();
    public readonly restartAgentRun = output<void>();
}

@Component({ selector: 'app-issue-activity-feed', template: '', standalone: true })
class ActivityFeedStub {
    public readonly idIssue = input<number | null>(null);
    public readonly idProject = input<number>(0);
    public readonly pendingTrack = input<Track | null>(null);
    public readonly agentRun = input<unknown>(null);
    public readonly approveAgentRun = output<void>();
    public readonly approveAgentRunMockup = output<unknown>();
}

describe('IssueDetailPage split dialog (browser)', () => {
    const issue = Fixtures.issue({ idIssue: 10, idIssuePublic: 5, idProject: 1, title: 'X' });
    let fixture: ComponentFixture<IssueDetailPage>;
    let setContext: ReturnType<typeof vi.fn>;

    function issueInfo(): IssueInfoStub {
        return fixture.debugElement.query(By.directive(IssueInfoStub))
            .componentInstance as IssueInfoStub;
    }

    function dialog(): SplitDialogStub | null {
        const el = fixture.debugElement.query(By.directive(SplitDialogStub));
        return el ? (el.componentInstance as SplitDialogStub) : null;
    }

    beforeEach(async () => {
        setContext = vi.fn();
        await TestBed.configureTestingModule({
            imports: [
                TranslateModule.forRoot(),
                UiModule,
                IssueInfoStub,
                ActivityFeedStub,
                SplitDialogStub
            ],
            declarations: [IssueDetailPage, SplitDialogHostComponent],
            providers: [
                provideRouter([]),
                {
                    provide: ActivatedRoute,
                    useValue: {
                        paramMap: of(convertToParamMap({ idProject: '1', idIssuePublic: '5' }))
                    }
                },
                { provide: IssueApi, useValue: { loadOne$: () => of(issue) } },
                { provide: ProjectStore, useValue: { project$: of({ idProject: 1 }) } },
                { provide: NoticeService, useValue: { issue$: new Subject<any>() } },
                {
                    provide: I18nService,
                    useValue: { instant: (k: string, _p?: Record<string, unknown>) => k }
                },
                { provide: CommandPaletteService, useValue: { setContext } }
            ]
        })
            // The real providers stay; only the agent-run store is swapped for a
            // dummy — the split dialog does not read it.
            .overrideComponent(IssueDetailPage, {
                set: {
                    providers: [
                        {
                            provide: AgentRunStore,
                            useValue: { run: () => null, loadForIssue: vi.fn() }
                        },
                        LinkScope
                    ]
                }
            })
            .compileComponents();

        fixture = TestBed.createComponent(IssueDetailPage);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();
    });

    it('shows no dialog until issue info requests a split', () => {
        expect(dialog()).toBeNull();
    });

    it('opens the dialog with the requested issue when issue info requests a split', () => {
        issueInfo().splitRequested.emit(issue);
        fixture.detectChanges();

        const stub = dialog();
        expect(stub).not.toBeNull();
        expect(stub!.issue()).toBe(issue);
        expect(stub!.idProject()).toBe(1);
    });

    it('closes the dialog when the split is cancelled', () => {
        issueInfo().splitRequested.emit(issue);
        fixture.detectChanges();
        dialog()!.cancelled.emit();
        fixture.detectChanges();

        expect(dialog()).toBeNull();
    });

    it('closes the dialog when the split is accepted', () => {
        issueInfo().splitRequested.emit(issue);
        fixture.detectChanges();
        dialog()!.accepted.emit([issue] as Issue[]);
        fixture.detectChanges();

        expect(dialog()).toBeNull();
    });
});

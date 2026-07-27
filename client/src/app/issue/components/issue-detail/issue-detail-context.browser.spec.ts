import { describe, it, expect, vi, beforeEach } from 'vitest';
import { EMPTY, of } from 'rxjs';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { IssueDetailPage } from './issue-detail.page';
import { IssueService } from '../../issue.service';
import { ProjectStore } from 'src/app/project/project.store';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { CommandPaletteService } from 'src/app/core/command/command-palette.service';
import { AgentRunStore } from 'src/app/agent/store/agent-run.store';
import { Issue } from '../../model/issue.model';

const issue = {
    idProject: 1,
    idIssue: 10,
    idIssuePublic: 5,
    title: 'X',
    idState: 1,
    idSeverity: null,
    description: '',
    tracked: 0
} as Issue;

describe('IssueDetailPage palette context', () => {
    let setContext: any;
    beforeEach(() => {
        setContext = vi.fn();
        TestBed.configureTestingModule({
            declarations: [IssueDetailPage],
            providers: [
                {
                    provide: ActivatedRoute,
                    useValue: {
                        paramMap: of(convertToParamMap({ idProject: '1', idIssuePublic: '5' }))
                    }
                },
                {
                    provide: IssueService,
                    useValue: { loadIssue: () => of(issue), toIssue: (x: Issue) => x }
                },
                { provide: ProjectStore, useValue: { project$: of({ idProject: 1 }) } },
                { provide: NoticeService, useValue: { issue$: EMPTY } },
                { provide: CommandPaletteService, useValue: { setContext } }
            ]
        }).overrideComponent(IssueDetailPage, {
            set: {
                template: '',
                providers: [{ provide: AgentRunStore, useValue: { loadForIssue: vi.fn() } }]
            }
        });
    });

    it('pushes {idProject, issue} when the issue loads', () => {
        TestBed.createComponent(IssueDetailPage).detectChanges();
        expect(setContext).toHaveBeenCalledWith({ idProject: 1, issue });
    });

    it('clears the issue (keeps idProject) on destroy', () => {
        const f = TestBed.createComponent(IssueDetailPage);
        f.detectChanges();
        setContext.mockClear();
        f.destroy();
        expect(setContext).toHaveBeenCalledWith({ idProject: 1, issue: null });
    });
});

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { EMPTY, of, Subject } from 'rxjs';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { IssueDetailPage } from './issue-detail.page';
import { IssueService } from '../../issue.service';
import { ProjectStore } from 'src/app/project/project.store';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { CommandPaletteService } from 'src/app/core/command/command-palette.service';
import { BrowserTitleService, DEFAULT_TITLE } from 'src/app/core/browser-title.service';
import { AgentRunStore } from 'src/app/agent/store/agent-run.store';
import { Issue } from '../../model/issue.model';

const issue: Issue = {
    idProject: 1,
    idIssue: 10,
    idIssuePublic: 5,
    title: 'X',
    idState: 1,
    idSeverity: null,
    description: '',
    tracked: 0
};

describe('IssueDetailPage title', () => {
    let setContext: any;
    let issue$: Subject<any>;
    beforeEach(() => {
        setContext = vi.fn();
        issue$ = new Subject<any>();
        document.title = DEFAULT_TITLE;
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
                { provide: NoticeService, useValue: { issue$ } },
                { provide: CommandPaletteService, useValue: { setContext } }
            ]
        }).overrideComponent(IssueDetailPage, {
            set: {
                template: '',
                providers: [{ provide: AgentRunStore, useValue: { loadForIssue: vi.fn() } }]
            }
        });
    });

    it('sets document.title to issue number and title after load', () => {
        TestBed.createComponent(IssueDetailPage).detectChanges();
        expect(document.title).toBe('#5 X · RuRdesk');
    });

    it('restores document.title to default on destroy', () => {
        const f = TestBed.createComponent(IssueDetailPage);
        f.detectChanges();
        f.destroy();
        expect(document.title).toBe(DEFAULT_TITLE);
    });

    it('updates document.title when the issue title changes via notice', () => {
        const f = TestBed.createComponent(IssueDetailPage);
        f.detectChanges();
        const renamed: Issue = { ...issue, title: 'Renamed' };
        issue$.next({ payload: renamed, subject: 'issue', action: 'update' });
        f.detectChanges();
        expect(document.title).toBe('#5 Renamed · RuRdesk');
    });

    it('truncates a long issue title while keeping the number visible', () => {
        const longIssue: Issue = { ...issue, title: 'a'.repeat(80) };
        TestBed.configureTestingModule({
            providers: [
                {
                    provide: IssueService,
                    useValue: { loadIssue: () => of(longIssue), toIssue: (x: Issue) => x }
                }
            ]
        });
        TestBed.createComponent(IssueDetailPage).detectChanges();
        expect(document.title.startsWith('#5 ')).toBe(true);
        expect(document.title).toContain('… · RuRdesk');
    });
});

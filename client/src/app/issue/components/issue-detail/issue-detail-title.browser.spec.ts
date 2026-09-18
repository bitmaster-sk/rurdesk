import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { of, Subject } from 'rxjs';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { IssueDetailPage } from './issue-detail.page';
import { IssueApi } from '../../api/issue.api.service';
import { ProjectStore } from 'src/app/project/project.store';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { CommandPaletteService } from 'src/app/core/command/command-palette.service';
import { DEFAULT_TITLE } from 'src/app/core/browser-title.service';
import { AgentRunStore } from 'src/app/agent/store/agent-run.store';
import { Issue } from '../../model/issue.model';
import { Fixtures } from 'src/testing/fixtures';

const issue: Issue = Fixtures.issue({ idIssue: 10, idIssuePublic: 5, title: 'X', idState: 1 });

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
                    provide: IssueApi,
                    useValue: { loadOne$: () => of(issue) }
                },
                { provide: ProjectStore, useValue: { project$: of({ idProject: 1 }) } },
                { provide: NoticeService, useValue: { issue$ } },
                {
                    provide: I18nService,
                    useValue: { instant: (k: string, _p?: Record<string, unknown>) => k }
                },
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
                    provide: IssueApi,
                    useValue: { loadOne$: () => of(longIssue) }
                }
            ]
        });
        TestBed.createComponent(IssueDetailPage).detectChanges();
        expect(document.title.startsWith('#5 ')).toBe(true);
        expect(document.title).toContain('… · RuRdesk');
    });
});

describe('IssueDetailPage clone pre-fill', () => {
    const sourceIssue: Issue = Fixtures.issue({
        idIssue: 10,
        idIssuePublic: 5,
        title: 'Original',
        description: 'desc',
        idState: 2,
        idSeverity: 3,
        idIssueType: 4,
        assignedTo: 7,
        estimated: 3600,
        points: 5,
        idProject: 1
    });

    function setupCloneRoute(): void {
        TestBed.configureTestingModule({
            declarations: [IssueDetailPage],
            providers: [
                {
                    provide: ActivatedRoute,
                    useValue: {
                        paramMap: of(convertToParamMap({ idProject: '1', idIssuePublic: '0' }))
                    }
                },
                {
                    provide: IssueApi,
                    useValue: { loadOne$: () => of(sourceIssue) }
                },
                { provide: ProjectStore, useValue: { project$: of({ idProject: 1 }) } },
                { provide: NoticeService, useValue: { issue$: new Subject() } },
                {
                    provide: I18nService,
                    useValue: {
                        instant: (k: string, p?: Record<string, unknown>) =>
                            k === 'ISSUE.COPY_SUFFIX' ? `${p!.title as string} (copy)` : k
                    }
                },
                { provide: CommandPaletteService, useValue: { setContext: vi.fn() } }
            ]
        }).overrideComponent(IssueDetailPage, {
            set: {
                template: '',
                providers: [{ provide: AgentRunStore, useValue: { loadForIssue: vi.fn() } }]
            }
        });
    }

    afterEach(() => {
        history.replaceState(null, '');
    });

    it('pre-fills the draft from history.state.cloneSource with a "(copy)" title suffix', () => {
        history.replaceState({ cloneSource: sourceIssue }, '');
        setupCloneRoute();

        const f = TestBed.createComponent(IssueDetailPage);
        f.detectChanges();

        const issue = (f.componentInstance as unknown as { issue: () => Issue | null }).issue();
        expect(issue?.title).toBe('Original (copy)');
        expect(issue?.description).toBe('desc');
        expect(issue?.idState).toBe(2);
        expect(issue?.idSeverity).toBe(3);
        expect(issue?.idIssueType).toBe(4);
        expect(issue?.assignedTo).toBe(7);
        expect(issue?.estimated).toBe(3600);
        expect(issue?.points).toBe(5);
    });

    it('returns a blank draft when there is no cloneSource in history state', () => {
        history.replaceState({}, '');
        setupCloneRoute();

        const f = TestBed.createComponent(IssueDetailPage);
        f.detectChanges();

        const issue = (f.componentInstance as unknown as { issue: () => Issue | null }).issue();
        expect(issue?.title).toBe('');
        expect(issue?.description).toBe('');
        expect(issue?.idState).toBeNull();
    });
});

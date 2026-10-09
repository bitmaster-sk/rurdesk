import { Component, input, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { DatePipe } from '@angular/common';
import { Router, RouterModule, provideRouter } from '@angular/router';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { Observable, Subject, of, throwError } from 'rxjs';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { User } from 'src/app/auth/model/user.model';
import {
    TablerIconStub,
    UiButtonStub,
    UiDialogStub,
    UiLoaderStub,
    UiTagStub
} from 'src/testing/stubs';
import { WikiApi } from '../../api/wiki.api.service';
import { WikiProposalKind } from '../../constants/wiki-proposal-kind.enum';
import { WikiProposalState } from '../../constants/wiki-proposal-state.enum';
import { WikiSpaceKind } from '../../constants/wiki-space-kind.enum';
import {
    WikiProposal,
    WikiProposalAcceptResult,
    WikiProposalDetail
} from '../../model/wiki-proposal.model';
import { WikiTreeStore } from '../../store/wiki-tree.store';
import { WikiProposalDetailComponent } from './wiki-proposal-detail.component';

@Component({ selector: 'app-diff-viewer', template: '', standalone: true })
class DiffViewerStub {
    public readonly rawPatch = input<string | null>(null);
}

function proposal(overrides: Partial<WikiProposal> = {}): WikiProposal {
    return {
        idProposal: 5,
        idRun: 19,
        idUserAgent: 3,
        idIssue: 175,
        idIssuePublic: 175,
        issueTitle: 'Reuse the branch',
        idProject: 7,
        idSpace: 2,
        spaceKind: WikiSpaceKind.Project,
        kind: WikiProposalKind.Update,
        idPage: 40,
        slug: 'agent-run',
        title: 'Agent run',
        summary: '',
        body: 'new',
        idParent: null,
        parentSlug: null,
        parentTitle: null,
        reason: 'The branch name changed.',
        baseVersion: 4,
        agentAccess: null,
        state: WikiProposalState.Ready,
        decidedBy: null,
        decidedAt: null,
        decisionNote: null,
        resultVersion: null,
        pageVersion: 4,
        pageTitle: 'Agent run',
        pageParentTitle: null,
        isPageLive: true,
        createAt: '2026-10-08T10:00:00Z',
        updateAt: '2026-10-08T10:00:00Z',
        ...overrides
    };
}

describe('WikiProposalDetailComponent', () => {
    let fixture: ComponentFixture<WikiProposalDetailComponent>;
    let detail: WikiProposalDetail;
    let accept: ReturnType<typeof vi.fn<() => Observable<WikiProposalAcceptResult>>>;
    let reject: ReturnType<typeof vi.fn<(id: number, reason: string) => Observable<WikiProposal>>>;
    let toast: {
        showSuccess: ReturnType<typeof vi.fn>;
        showInfo: ReturnType<typeof vi.fn>;
        showError: ReturnType<typeof vi.fn>;
    };
    let canEdit: ReturnType<typeof signal<boolean>>;
    let reload: ReturnType<typeof vi.fn>;
    let notices: Subject<{ payload: { idProject: number; idIssue: number } }>;

    beforeEach(async () => {
        detail = { proposal: proposal(), diff: '--- a\n+++ b\n', conflicts: 0 };
        accept = vi.fn(() =>
            of({
                proposal: proposal({ state: WikiProposalState.Accepted, decidedBy: 1 }),
                page: null,
                mergedFrom: null
            })
        );
        reject = vi.fn((_: number, reason: string) =>
            of(proposal({ state: WikiProposalState.Rejected, decisionNote: reason }))
        );
        toast = { showSuccess: vi.fn(), showInfo: vi.fn(), showError: vi.fn() };
        canEdit = signal(true);
        reload = vi.fn();
        notices = new Subject();

        await TestBed.configureTestingModule({
            declarations: [WikiProposalDetailComponent],
            imports: [
                TranslateModule.forRoot(),
                RouterModule,
                DatePipe,
                TablerIconStub,
                UiButtonStub,
                UiDialogStub,
                UiLoaderStub,
                UiTagStub,
                DiffViewerStub
            ],
            providers: [
                provideRouter([]),
                {
                    provide: WikiApi,
                    useValue: {
                        loadProposal$: () => of(detail),
                        acceptProposal$: accept,
                        rejectProposal$: reject
                    }
                },
                { provide: ToastNotificationService, useValue: toast },
                { provide: NoticeService, useValue: { wikiProposal$: notices.asObservable() } },
                {
                    provide: WikiTreeStore,
                    useValue: {
                        projectSpace: () => ({ canEdit: canEdit() }),
                        sharedSpace: () => ({ canEdit: false }),
                        reload
                    }
                },
                {
                    provide: ProjectMemberStore,
                    useValue: {
                        usersMap$: of(
                            new Map<number, User>([
                                [3, { idUser: 3, name: 'Kimi', email: '', colorAvatarBg: '' }],
                                [1, { idUser: 1, name: 'Tomas', email: '', colorAvatarBg: '' }]
                            ])
                        )
                    }
                }
            ]
        }).compileComponents();
    });

    function render(): HTMLElement {
        fixture = TestBed.createComponent(WikiProposalDetailComponent);
        fixture.componentRef.setInput('idProject', 7);
        fixture.componentRef.setInput('idProposal', 5);
        fixture.detectChanges();
        return fixture.nativeElement as HTMLElement;
    }

    function byTestId(el: HTMLElement, testId: string): HTMLElement | null {
        return el.querySelector<HTMLElement>(`[data-testid="${testId}"]`);
    }

    it('accepts a ready proposal and shows who decided it', () => {
        const el = render();
        const decided = vi.fn();
        fixture.componentInstance.decided.subscribe(decided);

        expect(byTestId(el, 'wiki-proposal-reason')?.textContent).toContain(
            'The branch name changed.'
        );
        expect(el.textContent).toContain('Kimi');
        byTestId(el, 'wiki-proposal-accept')?.click();
        fixture.detectChanges();

        expect(accept).toHaveBeenCalledWith(5, {});
        expect(toast.showSuccess).toHaveBeenCalledWith('WIKI.PROPOSAL.ACCEPTED');
        expect(reload).toHaveBeenCalled();
        expect(decided).toHaveBeenCalledOnce();
        expect(byTestId(el, 'wiki-proposal-state')?.textContent).toContain(
            'WIKI.PROPOSAL.STATE.ACCEPTED'
        );
        expect(byTestId(el, 'wiki-proposal-accept')).toBeNull();
    });

    it('approves an open proposal before the merge and then only offers to edit it', () => {
        detail = { ...detail, proposal: proposal({ state: WikiProposalState.Open }) };
        accept.mockReturnValue(
            of({
                proposal: proposal({ state: WikiProposalState.Approved, decidedBy: 1 }),
                page: null,
                mergedFrom: null
            })
        );
        const el = render();

        expect(byTestId(el, 'wiki-proposal-accept')?.textContent).toContain(
            'WIKI.PROPOSAL.APPROVE'
        );
        byTestId(el, 'wiki-proposal-accept')?.click();
        fixture.detectChanges();

        expect(toast.showSuccess).toHaveBeenCalledWith('WIKI.PROPOSAL.APPROVED');
        expect(byTestId(el, 'wiki-proposal-accept')).toBeNull();
        expect(byTestId(el, 'wiki-proposal-edit')?.textContent).toContain('WIKI.PROPOSAL.EDIT');
        expect(byTestId(el, 'wiki-proposal-reject')).not.toBeNull();
        expect(byTestId(el, 'wiki-proposal-decision')?.textContent).toContain(
            'WIKI.PROPOSAL.APPROVED_BY'
        );
    });

    it('sends a change that no longer merges to the editor to resolve', () => {
        detail = { ...detail, proposal: proposal({ state: WikiProposalState.NeedsResolving }) };
        const el = render();

        expect(byTestId(el, 'wiki-proposal-accept')).toBeNull();
        expect(byTestId(el, 'wiki-proposal-edit')?.textContent).toContain('WIKI.PROPOSAL.RESOLVE');
        expect(byTestId(el, 'wiki-proposal-resolve-hint')).not.toBeNull();
    });

    it('offers no decision to someone who cannot edit the wiki', () => {
        canEdit.set(false);
        const el = render();

        expect(byTestId(el, 'wiki-proposal-accept')).toBeNull();
    });

    it('opens the editor when the page changed and the edits overlap', () => {
        accept.mockReturnValue(
            throwError(() => ({ status: 409, error: { merge: { chunks: [], conflicts: 1 } } }))
        );
        const el = render();
        const navigate = vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);

        byTestId(el, 'wiki-proposal-accept')?.click();

        expect(toast.showInfo).toHaveBeenCalledWith('WIKI.PROPOSAL.CONFLICT');
        expect(navigate).toHaveBeenCalledWith(
            ['/project', 7, 'wiki', 'project', 'agent-run', 'edit'],
            { queryParams: { proposal: 5 } }
        );
    });

    it('tells whether accepting has to merge with later edits', () => {
        detail = { ...detail, proposal: proposal({ pageVersion: 6 }), conflicts: 0 };
        const el = render();

        expect(byTestId(el, 'wiki-proposal-merge-hint')?.textContent).toContain(
            'WIKI.PROPOSAL.CHANGED_MERGE'
        );
    });

    it('rejects with the reason the person typed', () => {
        const el = render();

        byTestId(el, 'wiki-proposal-reject')?.click();
        fixture.detectChanges();
        const reason = byTestId(el, 'wiki-proposal-reject-reason') as HTMLTextAreaElement;
        reason.value = '  we keep tabs  ';
        reason.dispatchEvent(new Event('input'));
        byTestId(el, 'wiki-proposal-reject-confirm')?.click();
        fixture.detectChanges();

        expect(reject).toHaveBeenCalledWith(5, 'we keep tabs');
        expect(byTestId(el, 'wiki-proposal-decision')?.textContent).toContain('we keep tabs');
    });

    it('describes a move between parents in words', () => {
        const translate = TestBed.inject(TranslateService);
        translate.setTranslation('en', {
            WIKI: {
                MOVE: { ROOT: 'Top level' },
                PROPOSAL: { MOVE_TEXT: 'Move {{page}} from {{from}} under {{to}}.' }
            }
        });
        translate.use('en');
        detail = {
            proposal: proposal({
                kind: WikiProposalKind.Move,
                body: null,
                parentSlug: 'ops',
                parentTitle: 'Operations'
            }),
            diff: '',
            conflicts: 0
        };
        const el = render();

        expect(byTestId(el, 'wiki-proposal-move')?.textContent?.trim()).toBe(
            'Move Agent run from Top level under Operations.'
        );
        expect(byTestId(el, 'wiki-proposal-edit')).toBeNull();
    });

    it('refreshes when the pull request of its task is merged', async () => {
        detail = { ...detail, proposal: proposal({ state: WikiProposalState.Approved }) };
        const el = render();
        expect(byTestId(el, 'wiki-proposal-accept')).toBeNull();

        detail = { ...detail, proposal: proposal({ state: WikiProposalState.Ready }) };
        notices.next({ payload: { idProject: 7, idIssue: 999 } });
        notices.next({ payload: { idProject: 7, idIssue: 175 } });
        await new Promise(resolve => setTimeout(resolve, 350));
        fixture.detectChanges();

        expect(byTestId(el, 'wiki-proposal-accept')).not.toBeNull();
    });
});

import { describe, expect, it } from 'vitest';
import { WikiProposalAction } from '../constants/wiki-proposal-action.enum';
import { WikiProposalKind } from '../constants/wiki-proposal-kind.enum';
import { WikiProposalState } from '../constants/wiki-proposal-state.enum';
import { WikiSpaceKind } from '../constants/wiki-space-kind.enum';
import { WikiProposal } from '../model/wiki-proposal.model';
import { WikiProposalConverter } from './wiki-proposal.converter';

function proposal(overrides: Partial<WikiProposal>): WikiProposal {
    return {
        idProposal: 5,
        idRun: 19,
        idUserAgent: 3,
        createBy: 3,
        authorName: 'Kimi',
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
        body: 'new text',
        idParent: null,
        parentSlug: null,
        parentTitle: null,
        reason: 'the branch name changed',
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

describe('WikiProposalConverter', () => {
    it('puts what needs a person first and keeps approved ones apart, leaving settled ones out', () => {
        const groups = WikiProposalConverter.toGroups(
            [
                proposal({ idProposal: 1, state: WikiProposalState.Open }),
                proposal({ idProposal: 2, state: WikiProposalState.Ready }),
                proposal({ idProposal: 3, state: WikiProposalState.NeedsResolving }),
                proposal({ idProposal: 4, state: WikiProposalState.Approved }),
                proposal({ idProposal: 5, state: WikiProposalState.Accepted }),
                proposal({ idProposal: 6, state: WikiProposalState.Discarded })
            ],
            null
        );

        expect(groups.toReview.map(item => item.idProposal)).toEqual([3, 2, 1]);
        expect(groups.approved.map(item => item.idProposal)).toEqual([4]);
    });

    it('filters by space', () => {
        const groups = WikiProposalConverter.toGroups(
            [
                proposal({ idProposal: 1 }),
                proposal({ idProposal: 2, spaceKind: WikiSpaceKind.Instance })
            ],
            WikiSpaceKind.Instance
        );

        expect(groups.toReview.map(item => item.idProposal)).toEqual([2]);
    });

    it('approves before the merge and publishes after it', () => {
        expect(WikiProposalConverter.toAction(WikiProposalState.Open)).toBe(
            WikiProposalAction.Approve
        );
        expect(WikiProposalConverter.toAction(WikiProposalState.Approved)).toBe(
            WikiProposalAction.Approve
        );
        expect(WikiProposalConverter.toAction(WikiProposalState.Ready)).toBe(
            WikiProposalAction.Publish
        );
        expect(WikiProposalConverter.toAction(WikiProposalState.NeedsResolving)).toBe(
            WikiProposalAction.Publish
        );
        expect(WikiProposalConverter.toAction(WikiProposalState.Accepted)).toBeNull();
        expect(WikiProposalConverter.toAction(WikiProposalState.Discarded)).toBeNull();
    });

    it('edits an update in the page editor and a new page in the create form', () => {
        expect(WikiProposalConverter.toEditLink(proposal({}), 7)).toEqual({
            commands: ['/project', 7, 'wiki', 'project', 'agent-run', 'edit'],
            queryParams: { proposal: 5 }
        });
        expect(
            WikiProposalConverter.toEditLink(
                proposal({
                    kind: WikiProposalKind.Create,
                    idPage: null,
                    isPageLive: false,
                    spaceKind: WikiSpaceKind.Instance
                }),
                7
            )
        ).toEqual({
            commands: ['/project', 7, 'wiki', 'new'],
            queryParams: { proposal: 5, space: 'instance' }
        });
    });

    it('offers no editor for moves, deletes and pages that are gone', () => {
        expect(
            WikiProposalConverter.toEditLink(proposal({ kind: WikiProposalKind.Move }), 7)
        ).toBeNull();
        expect(
            WikiProposalConverter.toEditLink(proposal({ kind: WikiProposalKind.Delete }), 7)
        ).toBeNull();
        expect(WikiProposalConverter.toEditLink(proposal({ isPageLive: false }), 7)).toBeNull();
        expect(WikiProposalConverter.toPageLink(proposal({ isPageLive: false }), 7)).toBeNull();
    });

    it('says where a rejection reason goes: to the agent, to the task, or nowhere', () => {
        expect(WikiProposalConverter.toRejectHintKey(proposal({}))).toBe(
            'WIKI.PROPOSAL.REJECT_HINT'
        );
        expect(WikiProposalConverter.toRejectHintKey(proposal({ idRun: null }))).toBe(
            'WIKI.PROPOSAL.REJECT_HINT_TASK'
        );
        expect(
            WikiProposalConverter.toRejectHintKey(proposal({ idRun: null, idIssue: null }))
        ).toBeNull();
    });

    it('names the run of an agent proposal and the author of any other in the editor', () => {
        expect(WikiProposalConverter.toEditingBanner(proposal({ idIssuePublic: 42 }))).toEqual({
            key: 'WIKI.PROPOSAL.EDITING',
            params: { issue: 42, run: 19 }
        });
        expect(
            WikiProposalConverter.toEditingBanner(proposal({ idRun: null, authorName: 'Tomas' }))
        ).toEqual({ key: 'WIKI.PROPOSAL.EDITING_PERSON', params: { name: 'Tomas' } });
    });

    it('names the author and words the reason for an agent or a person', () => {
        expect(WikiProposalConverter.toAuthorName(proposal({}))).toBe('Kimi');
        expect(WikiProposalConverter.toAuthorName(proposal({ authorName: null }))).toBe('—');
        expect(WikiProposalConverter.toReasonKey(proposal({}))).toBe('WIKI.PROPOSAL.REASON');
        expect(WikiProposalConverter.toReasonKey(proposal({ idRun: null }))).toBe(
            'WIKI.PROPOSAL.REASON_PERSON'
        );
    });
});

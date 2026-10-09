import { WikiProposalAction } from '../constants/wiki-proposal-action.enum';
import { WikiProposalKind } from '../constants/wiki-proposal-kind.enum';
import { WikiProposalState } from '../constants/wiki-proposal-state.enum';
import { WikiSpaceKind } from '../constants/wiki-space-kind.enum';
import {
    WikiProposalEditLink,
    WikiProposalGroups,
    WikiProposalTag
} from '../entity/wiki-proposal-view.entity';
import { WikiProposal } from '../model/wiki-proposal.model';

const KIND_TAGS: Record<WikiProposalKind, WikiProposalTag> = {
    [WikiProposalKind.Create]: { labelKey: 'WIKI.PROPOSAL.KIND.CREATE', severity: 'success' },
    [WikiProposalKind.Update]: { labelKey: 'WIKI.PROPOSAL.KIND.UPDATE', severity: 'info' },
    [WikiProposalKind.Move]: { labelKey: 'WIKI.PROPOSAL.KIND.MOVE', severity: 'secondary' },
    [WikiProposalKind.Delete]: { labelKey: 'WIKI.PROPOSAL.KIND.DELETE', severity: 'danger' }
};

const STATE_TAGS: Record<WikiProposalState, WikiProposalTag> = {
    [WikiProposalState.Open]: { labelKey: 'WIKI.PROPOSAL.STATE.OPEN', severity: 'info' },
    [WikiProposalState.Ready]: { labelKey: 'WIKI.PROPOSAL.STATE.READY', severity: 'success' },
    [WikiProposalState.Approved]: {
        labelKey: 'WIKI.PROPOSAL.STATE.APPROVED',
        severity: 'primary'
    },
    [WikiProposalState.NeedsResolving]: {
        labelKey: 'WIKI.PROPOSAL.STATE.NEEDS_RESOLVING',
        severity: 'warn'
    },
    [WikiProposalState.Discarded]: {
        labelKey: 'WIKI.PROPOSAL.STATE.DISCARDED',
        severity: 'secondary'
    },
    [WikiProposalState.Accepted]: {
        labelKey: 'WIKI.PROPOSAL.STATE.ACCEPTED',
        severity: 'contrast'
    },
    [WikiProposalState.Rejected]: { labelKey: 'WIKI.PROPOSAL.STATE.REJECTED', severity: 'danger' }
};

const REVIEW_ORDER: WikiProposalState[] = [
    WikiProposalState.NeedsResolving,
    WikiProposalState.Ready,
    WikiProposalState.Open
];

const ACTIONS: Partial<Record<WikiProposalState, WikiProposalAction>> = {
    [WikiProposalState.Open]: WikiProposalAction.Approve,
    [WikiProposalState.Approved]: WikiProposalAction.Approve,
    [WikiProposalState.Ready]: WikiProposalAction.Publish,
    [WikiProposalState.NeedsResolving]: WikiProposalAction.Publish
};

export abstract class WikiProposalConverter {
    public static toKindTag(kind: WikiProposalKind): WikiProposalTag {
        return KIND_TAGS[kind];
    }

    public static toStateTag(state: WikiProposalState): WikiProposalTag {
        return STATE_TAGS[state];
    }

    public static toGroups(
        proposals: WikiProposal[],
        space: WikiSpaceKind | null
    ): WikiProposalGroups {
        const shown = proposals.filter(proposal => space === null || proposal.spaceKind === space);
        return {
            toReview: REVIEW_ORDER.flatMap(state =>
                shown.filter(proposal => proposal.state === state)
            ),
            approved: shown.filter(proposal => proposal.state === WikiProposalState.Approved)
        };
    }

    public static toAction(state: WikiProposalState): WikiProposalAction | null {
        return ACTIONS[state] ?? null;
    }

    public static toEditLink(
        proposal: WikiProposal,
        idProject: number
    ): WikiProposalEditLink | null {
        const queryParams = { proposal: proposal.idProposal };
        if (proposal.kind === WikiProposalKind.Create) {
            return {
                commands: ['/project', idProject, 'wiki', 'new'],
                queryParams: { ...queryParams, space: proposal.spaceKind }
            };
        }
        if (proposal.kind === WikiProposalKind.Update && proposal.isPageLive) {
            return {
                commands: [
                    '/project',
                    idProject,
                    'wiki',
                    proposal.spaceKind,
                    proposal.slug,
                    'edit'
                ],
                queryParams
            };
        }
        return null;
    }

    public static toPageLink(
        proposal: WikiProposal,
        idProject: number
    ): (string | number)[] | null {
        if (!proposal.isPageLive) {
            return null;
        }
        return ['/project', idProject, 'wiki', proposal.spaceKind, proposal.slug];
    }

    public static toProposalLink(proposal: WikiProposal): (string | number)[] {
        return ['/project', proposal.idProject, 'wiki', 'proposals', proposal.idProposal];
    }
}

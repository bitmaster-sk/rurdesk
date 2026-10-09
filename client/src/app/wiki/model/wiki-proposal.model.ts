import { WikiAgentAccess } from '../constants/wiki-agent-access.enum';
import { WikiProposalKind } from '../constants/wiki-proposal-kind.enum';
import { WikiProposalState } from '../constants/wiki-proposal-state.enum';
import { WikiSpaceKind } from '../constants/wiki-space-kind.enum';
import { WikiPage } from './wiki-page.model';

export interface WikiProposal {
    idProposal: number;
    idRun: number;
    idUserAgent: number;
    idIssue: number;
    idIssuePublic: number;
    issueTitle: string;
    idProject: number;
    idSpace: number;
    spaceKind: WikiSpaceKind;
    kind: WikiProposalKind;
    idPage: number | null;
    slug: string;
    title: string;
    summary: string;
    body: string | null;
    idParent: number | null;
    parentSlug: string | null;
    parentTitle: string | null;
    reason: string;
    baseVersion: number | null;
    agentAccess: WikiAgentAccess | null;
    state: WikiProposalState;
    decidedBy: number | null;
    decidedAt: string | null;
    decisionNote: string | null;
    resultVersion: number | null;
    pageVersion: number | null;
    pageTitle: string | null;
    pageParentTitle: string | null;
    isPageLive: boolean;
    createAt: string;
    updateAt: string;
}

export interface WikiProposalDetail {
    proposal: WikiProposal;
    diff: string;
    conflicts: number;
}

export interface WikiProposalAcceptRequest {
    baseVersion?: number;
    title?: string;
    summary?: string;
    body?: string;
    agentAccess?: WikiAgentAccess;
    note?: string;
}

export interface WikiProposalAcceptResult {
    proposal: WikiProposal;
    page: WikiPage | null;
    mergedFrom: number | null;
}

export interface WikiProposalNotice {
    idProject: number;
    idIssue: number;
}

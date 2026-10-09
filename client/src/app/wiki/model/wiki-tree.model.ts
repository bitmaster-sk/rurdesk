import { WikiAgentAccess } from '../constants/wiki-agent-access.enum';
import { WikiSpaceKind } from '../constants/wiki-space-kind.enum';

export interface WikiSpaceView {
    idSpace: number;
    kind: WikiSpaceKind;
    canEdit: boolean;
    canManage: boolean;
    idHomePage: number | null;
}

export interface WikiTreeNode {
    idPage: number;
    idSpace: number;
    idParent: number | null;
    slug: string;
    title: string;
    agentAccess: WikiAgentAccess;
    rank: string;
}

export interface WikiTree {
    spaces: WikiSpaceView[];
    nodes: WikiTreeNode[];
    alwaysTokens: number;
    tokenLimit: number;
    trashCount: number;
    proposalCount: number;
}

export interface WikiSearchHit {
    idPage: number;
    idSpace: number;
    slug: string;
    title: string;
    summary: string;
    snippet: string;
    score: number;
}

export interface WikiTrashItem {
    idPage: number;
    idSpace: number;
    idParent: number | null;
    title: string;
    deletedAt: string;
    deletedBy: number | null;
    descendants: number;
    parentAlive: boolean;
}

import { WikiAgentAccess } from '../constants/wiki-agent-access.enum';
import { WikiSpaceKind } from '../constants/wiki-space-kind.enum';
import { WikiIssueLinkSource } from '../constants/wiki-issue-link-source.enum';

export interface WikiPage {
    idPage: number;
    idSpace: number;
    idParent: number | null;
    slug: string;
    title: string;
    summary: string;
    body: string;
    agentAccess: WikiAgentAccess;
    rank: string;
    versionNo: number;
    deletedAt: string | null;
    createAt: string;
    updateAt: string;
    createBy: number | null;
    updateBy: number | null;
}

export interface WikiPageRef {
    idPage: number;
    idSpace: number;
    slug: string;
    title: string;
}

export interface WikiResolvedLink {
    shared: boolean;
    slug: string;
    idPage: number | null;
    idSpace: number;
    title: string;
}

export interface WikiPageIssue {
    idIssue: number;
    idIssuePublic: number;
    title: string;
}

export interface WikiPageDraft {
    idPage: number;
    idUser: number;
    baseVersion: number;
    title: string;
    summary: string;
    body: string;
    updateAt: string;
}

export interface WikiPageView {
    page: WikiPage;
    spaceKind: WikiSpaceKind;
    canEdit: boolean;
    canManage: boolean;
    backlinks: WikiPageRef[];
    issues: WikiPageIssue[];
    links: WikiResolvedLink[];
    draft: WikiPageDraft | null;
}

export interface WikiIssueLink {
    idPage: number;
    idSpace: number;
    spaceKind: WikiSpaceKind;
    slug: string;
    title: string;
    source: WikiIssueLinkSource;
}

export interface WikiEditor {
    idUser: number;
    name: string;
}

export interface WikiSavedNotice {
    idPage: number;
    idSpace: number;
    versionNo: number;
    idUser: number;
    userName: string;
}

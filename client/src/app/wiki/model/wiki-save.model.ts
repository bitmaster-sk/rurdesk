import { WikiAgentAccess } from '../constants/wiki-agent-access.enum';
import { WikiMergeChunkKind } from '../constants/wiki-merge-chunk-kind.enum';
import { WikiSpaceKind } from '../constants/wiki-space-kind.enum';
import { WikiPage } from './wiki-page.model';

export interface WikiMergeChunk {
    kind: WikiMergeChunkKind;
    lines?: string[];
    base?: string[];
    mine?: string[];
    theirs?: string[];
}

export interface WikiMergeResult {
    chunks: WikiMergeChunk[];
    conflicts: number;
}

export interface WikiSaveResult {
    page: WikiPage;
    mergedFrom: number | null;
}

export interface WikiConflict {
    current: WikiPage;
    title: string;
    summary: string;
    agentAccess: WikiAgentAccess;
    merge: WikiMergeResult;
}

export interface WikiMergePreview {
    currentVersion: number;
    title: string;
    summary: string;
    agentAccess: WikiAgentAccess;
    merge: WikiMergeResult;
}

export interface WikiMergePreviewRequest {
    baseVersion: number;
    title: string;
    summary: string;
    body: string;
    agentAccess: WikiAgentAccess;
}

export interface WikiCreateRequest {
    space: WikiSpaceKind;
    idParent: number | null;
    title: string;
    summary: string;
    body: string;
    agentAccess: WikiAgentAccess;
    note: string;
}

export interface WikiSaveRequest {
    baseVersion: number;
    title: string;
    summary: string;
    body: string;
    agentAccess: WikiAgentAccess;
    note: string;
}

export interface WikiMoveRequest {
    idParent: number | null;
    idPrev: number | null;
    idNext: number | null;
}

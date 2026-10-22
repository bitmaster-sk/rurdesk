import { Observable } from 'rxjs';

export enum HostType {
    GitHub = 'github',
    GitLab = 'gitlab',
    Gitea = 'gitea'
}

export enum MrState {
    Open = 'open',
    Merged = 'merged',
    Closed = 'closed'
}

export enum CiStatus {
    Pending = 'pending',
    Success = 'success',
    Failed = 'failed',
    Canceled = 'canceled',
    Skipped = 'skipped',
    Unknown = 'unknown'
}

export interface GitIntegrationRes {
    idGitIntegration: number;
    idProject: number;
    name: string;
    hostType: HostType;
    baseUrl: string;
    repoPath: string;
    baseBranch: string | null;
    createdAt: string;
    updatedAt: string;
}

export interface CreateGitIntegrationReq {
    name: string;
    hostType: HostType;
    baseUrl: string;
    repoPath: string;
    baseBranch: string | null;
    accessToken: string;
}

export enum GitAccessResult {
    Ok = 'ok',
    Unreachable = 'unreachable',
    InvalidToken = 'invalid_token',
    RepoNotFound = 'repo_not_found',
    ReadOnly = 'read_only'
}

export interface CheckGitIntegrationReq {
    idGitIntegration?: number;
    hostType: HostType;
    baseUrl: string;
    repoPath: string;
    accessToken?: string;
}

export interface GitAccessCheckRes {
    result: GitAccessResult;
    defaultBranch?: string;
}

export interface UpdateGitIntegrationReq {
    name: string;
    hostType: HostType;
    baseUrl: string;
    repoPath: string;
    baseBranch: string | null;
    accessToken?: string;
}

export interface MrDiffFile {
    oldPath: string;
    newPath: string;
    patch: string;
}

export interface MrDiff {
    headSha: string;
    files: MrDiffFile[];
}

export interface MrStatus {
    state: MrState;
    approved: boolean;
    ciStatus: CiStatus;
    webUrl: string;
    headSha: string;
}

export type FileContentLoader = (path: string, ref: string) => Observable<string[]>;

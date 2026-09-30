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
    createdAt: string;
    updatedAt: string;
}

export interface CreateGitIntegrationReq {
    name: string;
    hostType: HostType;
    baseUrl: string;
    repoPath: string;
    accessToken: string;
}

export interface UpdateGitIntegrationReq {
    name: string;
    hostType: HostType;
    baseUrl: string;
    repoPath: string;
    accessToken?: string;
}

export interface MrDiffFile {
    oldPath: string;
    newPath: string;
    patch: string;
    isDeleted: boolean;
}

export interface MrDiff {
    headSha: string;
    baseSha: string;
    files: MrDiffFile[];
}

export interface MrStatus {
    state: MrState;
    approved: boolean;
    ciStatus: CiStatus;
    webUrl: string;
    headSha: string;
    baseSha: string;
}

export enum DiffExpandDirection {
    Up = 'up',
    Down = 'down'
}

export interface FileContentRequest {
    file: MrDiffFile;
    ref: string;
    direction: DiffExpandDirection;
    line: number;
    count: number;
}

export interface FileContentResponse {
    lines: string[];
    lineCount: number;
}

export type FileContentLoader = (
    req: FileContentRequest
) => import('rxjs').Observable<FileContentResponse>;

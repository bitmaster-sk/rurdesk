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
}

export interface MrDiff {
    headSha: string;
    files: MrDiffFile[];
}

export interface MrStatus {
    state: MrState;
    approved: boolean;
    ciStatus: CiStatus;
}

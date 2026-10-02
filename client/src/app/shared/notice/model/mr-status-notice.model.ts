// Mirrors api/internal/model.MrStatusNotice — payload for the 'mr_status'
// WebSocket notice pushed when an open manual MR's CI/approval/head changes.
export interface MrStatusNotice {
    idIssue: number;
    idGitIntegration: number;
    idMr: string;
    state: string;
    approved: boolean;
    ciStatus: string;
    webUrl: string;
    headSha: string;
}

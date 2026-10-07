import { WikiAgentAccess } from '../constants/wiki-agent-access.enum';

export interface WikiVersionSummary {
    versionNo: number;
    title: string;
    agentAccess: WikiAgentAccess;
    note: string;
    mergedFrom: number | null;
    createAt: string;
    createBy: number | null;
}

export interface WikiVersion extends WikiVersionSummary {
    idVersion: number;
    idPage: number;
    idParent: number | null;
    summary: string;
    body: string;
}

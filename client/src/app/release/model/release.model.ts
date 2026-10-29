import { AgentPhase } from 'src/app/agent/model/agent-phase.enum';
import { RunChoices } from 'src/app/agent/model/agent-run.model';
import { ReleaseOpenIssues } from '../constants/release-open-issues.enum';
import { ReleaseStatus } from '../constants/release-status.enum';
import { ReleaseTagAction } from '../constants/release-tag-action.enum';

export interface Release {
    idRelease: number;
    idProject: number;
    idTag: number | null;
    name: string;
    status: ReleaseStatus;
    notes: string;
    recreateTag: boolean;
    openIssues: ReleaseOpenIssues;
    issueCount: number;
    createAt: string;
    createBy: number | null;
    releasedAt: string | null;
    releasedBy: number | null;
}

export interface ReleaseView extends Release {
    tagName: string | null;
    idRunNotes: number | null;
    notesRunPhase: AgentPhase | null;
    notesRunError: string | null;
    isNotesPending: boolean;
}

export interface ReleaseIssue {
    idIssue: number;
    idIssuePublic: number;
    title: string;
    idIssueType: number | null;
    issueTypeName: string | null;
}

export interface ReleaseSection {
    idIssueType: number | null;
    name: string | null;
    issues: ReleaseIssue[];
}

export interface ReleaseDraft {
    sections: ReleaseSection[];
    openIssues: ReleaseIssue[];
}

export interface CreateReleaseReq {
    idTag: number;
    name: string;
}

export interface UpdateReleaseReq {
    name: string;
    recreateTag: boolean;
    openIssues: ReleaseOpenIssues;
    notes?: string;
}

export interface StartReleaseAgentReq extends Partial<RunChoices> {
    idUserAgent: number;
}

export interface DeleteReleaseReq {
    tag?: ReleaseTagAction;
    idTagTarget?: number;
}

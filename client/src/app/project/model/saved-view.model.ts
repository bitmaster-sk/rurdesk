import { IssueViewMode } from '../../issue/constants/issue-view-modes.enum';

export type SavedViewKanbanLayout = 'columns' | 'swimlane';

export interface SavedViewConfig {
    v: 1; // versioning for future-proofing
    title?: string;
    idsSeverity?: number[];
    severityUnset?: boolean;
    idsIssueType?: number[];
    issueTypeUnset?: boolean;
    idsState?: number[];
    stateUnset?: boolean;
    idsAssignedTo?: number[];
    assignedToUnset?: boolean;
    assignedToNull?: boolean;
    scheduledAtUnset?: boolean;
    createAtFrom?: string;
    createAtTo?: string;
    updateAtFrom?: string;
    updateAtTo?: string;
    createAtWithin?: string;
    updateAtWithin?: string;
    orderColumn?: string;
    orderDirection?: 'asc' | 'desc';
    kanbanLayout?: SavedViewKanbanLayout;
}

export interface SavedView {
    idSavedView: number;
    idProject: number;
    name: string;
    viewType: IssueViewMode;
    config: SavedViewConfig;
    isShared: boolean;
    createBy: number;
    updateAt: string;
}

/** What the write endpoints accept */
export interface SavedViewReq {
    name: string;
    viewType: IssueViewMode;
    config: SavedViewConfig;
    isShared: boolean;
}

export interface IssuesFilterParams {
    idProject?: number;
    title?: string;
    idsSeverity?: number[];
    severityUnset?: boolean;
    idsState?: number[];
    stateUnset?: boolean;
    idsAssignedTo?: number[];
    assignedToUnset?: boolean;
    idsIssuePublic?: number[];
    createAtFrom?: Date;
    createAtTo?: Date;
    updateAtFrom?: Date;
    updateAtTo?: Date;
    scheduledAtFrom?: Date;
    scheduledAtTo?: Date;
    scheduledAtUnset?: boolean;
    assignedToNull?: boolean;
    idSprint?: number | null;
    sprintUnset?: boolean;
}

export interface IssuesOrderParams {
    orderColumn: string;
    orderDirection: 'asc' | 'desc';
}

export interface IssuesFilter extends IssuesFilterParams, IssuesOrderParams {}

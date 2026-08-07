export interface IssuesFilterParams {
    idProject?: number;
    title?: string | null;
    idsSeverity?: number[];
    severityUnset?: boolean;
    idsState?: number[];
    stateUnset?: boolean;
    idsAssignedTo?: number[];
    assignedToUnset?: boolean;
    idsIssuePublic?: number[];
    createAtFrom?: Date | null;
    createAtTo?: Date | null;
    updateAtFrom?: Date | null;
    updateAtTo?: Date | null;
    createAtWithin?: string | null;
    updateAtWithin?: string | null;
    scheduledAtFrom?: Date | null;
    scheduledAtTo?: Date | null;
    scheduledAtUnset?: boolean;
    assignedToNull?: boolean;
    idSprint?: number | null;
    sprintUnset?: boolean;
}

export interface IssuesOrderParams {
    orderColumn: string;
    orderDirection: 'asc' | 'desc';
}

export interface IssuesFilter extends IssuesFilterParams, IssuesOrderParams {
    idProject: number;
}

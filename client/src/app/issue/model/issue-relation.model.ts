export interface ReadIssueRelationDto {
    idIssueRelation: number;
    relationType: string;
    relationSubType: string | null;
    lagMinutes: number | null;
    direction: string;
    label: string;
    inverseLabel: string;
    from: IssueRelationRef;
    to: IssueRelationRef;
    createdAt: string;
    createdBy: number;
}

export interface IssueRelationRef {
    idIssuePublic: number;
    title: string;
    idSeverity: number | null;
    idState: number | null;
    assignedTo: number | null;
    updateAt: string;
    qualityScore: number | null;
}

export interface CreateIssueRelationDto {
    idIssuePublicTo: number;
    relationType: string;
    relationSubType?: string | null;
    lagMinutes?: number | null;
}

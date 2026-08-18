import { IssueRelationDirection } from '../constants/issue-relation-direction.enum';
import { IssueRelationSubType } from '../constants/issue-relation-subtype.enum';
import { IssueRelationType } from '../constants/issue-relation-type.enum';

export interface ReadIssueRelationDto {
    idIssueRelation: number;
    relationType: IssueRelationType;
    relationSubType: IssueRelationSubType | null;
    lagMinutes: number | null;
    direction: IssueRelationDirection;
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
    relationType: IssueRelationType;
    relationSubType?: IssueRelationSubType | null;
    lagMinutes?: number | null;
}

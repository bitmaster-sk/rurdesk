import { IssueRelationRef, ReadIssueRelationDto } from '../../../model/issue-relation.model';

export type GanttRelation = Omit<ReadIssueRelationDto, 'from' | 'to'> & {
    from: Pick<IssueRelationRef, 'idIssuePublic'>;
    to: Pick<IssueRelationRef, 'idIssuePublic'>;
};

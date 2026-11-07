import { FormControl } from '@angular/forms';
import { Issue } from '../../../../../model/issue.model';

export type IssueFormValues = Pick<
    Issue,
    | 'idState'
    | 'idSeverity'
    | 'idIssueType'
    | 'title'
    | 'description'
    | 'assignedTo'
    | 'estimated'
    | 'points'
    | 'scheduledAt'
    | 'idsTag'
>;

export interface IssueInfoForm {
    idIssue: FormControl<number | null>;
    idIssuePublic: FormControl<number | null>;
    idProject: FormControl<number | null>;
    idState: FormControl<number | null>;
    idSeverity: FormControl<number | null>;
    idIssueType: FormControl<number | null>;
    title: FormControl<string | null>;
    description: FormControl<string | null>;
    assignedTo: FormControl<number | null>;
    estimated: FormControl<string | null>;
    points: FormControl<number | null>;
    scheduledAt: FormControl<Date | null>;
    idsTag: FormControl<number[]>;
}

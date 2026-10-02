import { IssueViewMode } from '../constants/issue-view-modes.enum';

export interface IssueListLocation {
    idProject: number;
    mode: IssueViewMode;
    idSavedView: number | null;
}

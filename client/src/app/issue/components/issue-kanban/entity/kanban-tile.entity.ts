import { User } from 'src/app/auth/model/user.model';
import { Issue } from '../../../model/issue.model';
import { ExtendedIssue } from '../../../model/extended-issue.model';

export interface KanbanTile extends ExtendedIssue {
    createUser: User | undefined;
    updateUser: User | undefined;
}

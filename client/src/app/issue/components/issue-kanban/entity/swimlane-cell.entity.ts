import { IssueState } from 'src/app/state/model/issue-state.model';
import { User } from 'src/app/auth/model/user.model';
import { KanbanTile } from './kanban-tile.entity';

export interface SwimlaneCell {
    state: IssueState;
    user: User | undefined;
    tiles: KanbanTile[];
    total: number;
    cursor: string | null;
    loading: boolean;
}

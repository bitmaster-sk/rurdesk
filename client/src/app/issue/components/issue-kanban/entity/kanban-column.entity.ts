import { IssueState } from 'src/app/state/model/issue-state.model';
import { KanbanTile } from './kanban-tile.entity';

export interface KanbanColumn {
    state: IssueState;
    tiles: KanbanTile[];
    total: number;
    cursor: string | null;
    loading: boolean;
}

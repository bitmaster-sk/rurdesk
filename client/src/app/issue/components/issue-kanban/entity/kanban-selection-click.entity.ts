import { Issue } from '../../../model/issue.model';
import { SelectionClickMode } from '../../../util/selection-click';

export interface TileSelectionClick {
    mode: SelectionClickMode;
    issue: Issue;
}

export interface KanbanSelectionClick {
    click: TileSelectionClick;
    siblings: readonly Issue[];
}

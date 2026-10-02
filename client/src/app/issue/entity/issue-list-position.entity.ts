import { SavedViewKanbanLayout } from 'src/app/project/model/saved-view.model';

export interface IssueListPosition {
    loadedCount: number;
    scrollTop: number;
    scrollLeft: number;
    date?: Date;
    calendarView?: string;
    kanbanLayout?: SavedViewKanbanLayout;
    idSprint?: number | null;
}

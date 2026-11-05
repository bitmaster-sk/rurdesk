import { describe, expect, it } from 'vitest';
import { Issue } from '../model/issue.model';
import { KanbanCursor, KanbanGrid } from './kanban-cursor';

const tile = (idIssuePublic: number): Issue => ({
    idIssue: idIssuePublic,
    idIssuePublic,
    idProject: 1,
    idState: 1,
    idSeverity: null,
    idIssueType: null,
    title: '',
    description: '',
    tracked: 0
});

const lane = (...ids: number[]): Issue[] => ids.map(tile);

const idOf = (issue: Issue | null): number | null => issue?.idIssuePublic ?? null;

const columns: KanbanGrid<Issue> = [[lane(1, 2, 3), lane(), lane(4, 5)]];

const swimlanes: KanbanGrid<Issue> = [
    [lane(1, 2), lane(3)],
    [lane(), lane()],
    [lane(4), lane(5, 6)]
];

describe('KanbanCursor', () => {
    it('starts on the first card when nothing is highlighted', () => {
        expect(idOf(KanbanCursor.vertical(columns, null, 1))).toBe(1);
        expect(idOf(KanbanCursor.across(columns, null, 1))).toBe(1);
        expect(KanbanCursor.vertical([[lane()]], null, 1)).toBeNull();
    });

    it('moves up and down within a column and stops at its ends', () => {
        expect(idOf(KanbanCursor.vertical(columns, 1, 1))).toBe(2);
        expect(idOf(KanbanCursor.vertical(columns, 3, 1))).toBe(3);
        expect(idOf(KanbanCursor.vertical(columns, 1, -1))).toBe(1);
    });

    it('moves across columns, skips empty ones and keeps the row position', () => {
        expect(idOf(KanbanCursor.across(columns, 3, 1))).toBe(5);
        expect(idOf(KanbanCursor.across(columns, 4, -1))).toBe(1);
        expect(idOf(KanbanCursor.across(columns, 5, 1))).toBe(5);
    });

    it('continues into the same column of the next swimlane, skipping empty cells', () => {
        expect(idOf(KanbanCursor.vertical(swimlanes, 2, 1))).toBe(4);
        expect(idOf(KanbanCursor.vertical(swimlanes, 5, -1))).toBe(3);
        expect(idOf(KanbanCursor.vertical(swimlanes, 4, -1))).toBe(2);
    });

    it('stays inside the cell when asked to, for extending a selection', () => {
        expect(idOf(KanbanCursor.vertical(swimlanes, 2, 1, true))).toBe(2);
        expect(KanbanCursor.laneOf(swimlanes, 6).map(idOf)).toEqual([5, 6]);
    });
});

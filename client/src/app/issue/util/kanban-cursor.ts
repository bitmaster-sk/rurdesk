import { Issue } from '../model/issue.model';

export type KanbanGrid<T extends Issue> = readonly (readonly (readonly T[])[])[];

interface CursorPosition {
    row: number;
    col: number;
    index: number;
}

export abstract class KanbanCursor {
    public static vertical<T extends Issue>(
        grid: KanbanGrid<T>,
        idIssuePublic: number | null,
        delta: 1 | -1,
        isWithinLane = false
    ): T | null {
        const at = KanbanCursor.locate(grid, idIssuePublic);
        if (!at) return KanbanCursor.first(grid);
        const lane = grid[at.row][at.col];
        const index = at.index + delta;
        if (index >= 0 && index < lane.length) return lane[index];
        if (isWithinLane) return lane[at.index];
        for (let row = at.row + delta; row >= 0 && row < grid.length; row += delta) {
            const next = grid[row][at.col] ?? [];
            if (next.length > 0) return delta > 0 ? next[0] : next[next.length - 1];
        }
        return lane[at.index];
    }

    public static across<T extends Issue>(
        grid: KanbanGrid<T>,
        idIssuePublic: number | null,
        delta: 1 | -1
    ): T | null {
        const at = KanbanCursor.locate(grid, idIssuePublic);
        if (!at) return KanbanCursor.first(grid);
        const lanes = grid[at.row];
        for (let col = at.col + delta; col >= 0 && col < lanes.length; col += delta) {
            const next = lanes[col];
            if (next.length > 0) return next[Math.min(at.index, next.length - 1)];
        }
        return lanes[at.col][at.index];
    }

    public static laneOf<T extends Issue>(
        grid: KanbanGrid<T>,
        idIssuePublic: number | null
    ): readonly T[] {
        const at = KanbanCursor.locate(grid, idIssuePublic);
        return at ? grid[at.row][at.col] : [];
    }

    private static first<T extends Issue>(grid: KanbanGrid<T>): T | null {
        for (const lanes of grid) {
            for (const lane of lanes) {
                if (lane.length > 0) return lane[0];
            }
        }
        return null;
    }

    private static locate<T extends Issue>(
        grid: KanbanGrid<T>,
        idIssuePublic: number | null
    ): CursorPosition | null {
        if (idIssuePublic === null) return null;
        for (let row = 0; row < grid.length; row++) {
            for (let col = 0; col < grid[row].length; col++) {
                const index = grid[row][col].findIndex(
                    tile => tile.idIssuePublic === idIssuePublic
                );
                if (index !== -1) return { row, col, index };
            }
        }
        return null;
    }
}

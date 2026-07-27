/**
 * Pure orthogonal-routing geometry for gantt dependency arrows.
 *
 * Layout invariant this relies on: every task occupies its own row, so the only
 * bar in the target row is the target itself. Horizontal segments therefore run
 * either inside the source/target row (safe) or along a row boundary (inside the
 * vertical BAR_GAP between bars — also safe). Vertical segments may still cross
 * bars in intermediate rows; avoiding that needs a full obstacle router, which
 * is out of scope.
 */

export const HORIZONTAL_OFFSET = 15;
const SAME_ROW_VERTICAL_DETOUR = 20;
/** Extra px per lane so arrows sharing a source/target fan out instead of overlapping. */
export const LANE_SPACING = 6;
/** Cap so many relations on one task don't push stubs absurdly far out. */
export const MAX_LANE = 3;

export type ArrowDirection = 'left' | 'right';

export interface ArrowEndpoints {
    sourceX: number;
    sourceY: number;
    targetX: number;
    targetY: number;
    exitDirection: ArrowDirection;
    enterDirection: ArrowDirection;
    sourceRowIndex: number;
    targetRowIndex: number;
    rowHeight: number;
    /** 0-based lane among arrows leaving the same task side (fans out stubs). */
    exitLane: number;
    /** 0-based lane among arrows entering the same task side. */
    enterLane: number;
}

export interface ArrowRoute {
    path: string;
    midX: number;
    midY: number;
}

export function routeArrow(e: ArrowEndpoints): ArrowRoute {
    const exitOff = HORIZONTAL_OFFSET + Math.min(e.exitLane, MAX_LANE) * LANE_SPACING;
    const enterOff = HORIZONTAL_OFFSET + Math.min(e.enterLane, MAX_LANE) * LANE_SPACING;
    const { sourceX: x1, sourceY: y1, targetX: x2, targetY: y2 } = e;
    const isSameRow = e.sourceRowIndex === e.targetRowIndex;

    // Same-side connections (SS / FF): one shared vertical anchor beside both bars
    if (e.exitDirection === e.enterDirection) {
        const anchorX =
            e.exitDirection === 'left' ? Math.min(x1, x2) - exitOff : Math.max(x1, x2) + exitOff;

        if (isSameRow) {
            const detourY = y1 - SAME_ROW_VERTICAL_DETOUR;
            return {
                path: `M ${x1} ${y1} L ${anchorX} ${y1} L ${anchorX} ${detourY} L ${x2} ${detourY} L ${x2} ${y2}`,
                midX: (anchorX + x2) / 2,
                midY: detourY
            };
        }
        return {
            path: `M ${x1} ${y1} L ${anchorX} ${y1} L ${anchorX} ${y2} L ${x2} ${y2}`,
            midX: anchorX,
            midY: (y1 + y2) / 2
        };
    }

    // Different-side connections (FS / SF)
    const exitX = e.exitDirection === 'right' ? x1 + exitOff : x1 - exitOff;
    const enterX = e.enterDirection === 'left' ? x2 - enterOff : x2 + enterOff;

    if (isSameRow) {
        const detourY = y1 - SAME_ROW_VERTICAL_DETOUR;
        return {
            path: `M ${x1} ${y1} L ${exitX} ${y1} L ${exitX} ${detourY} L ${enterX} ${detourY} L ${enterX} ${y2} L ${x2} ${y2}`,
            midX: (exitX + enterX) / 2,
            midY: detourY
        };
    }

    // Enough forward room → drop straight from the exit stub into the target row,
    // then run inside the target row (only the target bar lives there).
    const hasRoom = e.exitDirection === 'right' ? exitX <= enterX : exitX >= enterX;
    if (hasRoom) {
        return {
            path: `M ${x1} ${y1} L ${exitX} ${y1} L ${exitX} ${y2} L ${x2} ${y2}`,
            midX: exitX,
            midY: (y1 + y2) / 2
        };
    }

    // No room (typical back-to-back finish-to-start chain): S-route whose long
    // horizontal runs along the row boundary — the gap between bars — instead of
    // cutting through them.
    const boundaryY =
        y2 > y1 ? (e.sourceRowIndex + 1) * e.rowHeight : e.sourceRowIndex * e.rowHeight;
    return {
        path: `M ${x1} ${y1} L ${exitX} ${y1} L ${exitX} ${boundaryY} L ${enterX} ${boundaryY} L ${enterX} ${y2} L ${x2} ${y2}`,
        midX: (exitX + enterX) / 2,
        midY: boundaryY
    };
}

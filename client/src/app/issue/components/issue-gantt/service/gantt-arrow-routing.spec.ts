import {
    ArrowEndpoints,
    HORIZONTAL_OFFSET,
    LANE_SPACING,
    MAX_LANE,
    routeArrow
} from './gantt-arrow-routing';

const ROW_HEIGHT = 64;

function endpoints(overrides: Partial<ArrowEndpoints> = {}): ArrowEndpoints {
    return {
        sourceX: 100,
        sourceY: 32,
        targetX: 300,
        targetY: 96,
        exitDirection: 'right',
        enterDirection: 'left',
        sourceRowIndex: 0,
        targetRowIndex: 1,
        rowHeight: ROW_HEIGHT,
        exitLane: 0,
        enterLane: 0,
        ...overrides
    };
}

/** Parses "M x y L x y ..." into point tuples. */
function points(path: string): [number, number][] {
    return [...path.matchAll(/[ML] (-?[\d.]+) (-?[\d.]+)/g)].map(m => [Number(m[1]), Number(m[2])]);
}

describe('routeArrow', () => {
    it('routes finish-to-start with room via a drop into the target row', () => {
        const { path } = routeArrow(endpoints());
        expect(points(path)).toEqual([
            [100, 32],
            [100 + HORIZONTAL_OFFSET, 32],
            [100 + HORIZONTAL_OFFSET, 96],
            [300, 96]
        ]);
    });

    it('keeps the long horizontal of a back-to-back chain on the row boundary', () => {
        // Target starts exactly where the source ends — no forward room
        const { path } = routeArrow(endpoints({ targetX: 100 }));
        const pts = points(path);
        // Middle horizontal segment must sit on the row boundary (y = rowHeight),
        // i.e. in the gap between bars, not across a bar's interior.
        expect(pts[2][1]).toBe(ROW_HEIGHT);
        expect(pts[3][1]).toBe(ROW_HEIGHT);
        // Route still starts at the source and ends at the target handle
        expect(pts[0]).toEqual([100, 32]);
        expect(pts[pts.length - 1]).toEqual([100, 96]);
    });

    it('uses the boundary above the source when the target row is above', () => {
        const { path } = routeArrow(
            endpoints({
                sourceY: 96,
                sourceRowIndex: 1,
                targetY: 32,
                targetRowIndex: 0,
                targetX: 100
            })
        );
        const pts = points(path);
        expect(pts[2][1]).toBe(ROW_HEIGHT); // top edge of source row 1
    });

    it('fans out arrows sharing an exit side via lanes', () => {
        const inner = routeArrow(endpoints({ exitLane: 0 }));
        const outer = routeArrow(endpoints({ exitLane: 1 }));
        const innerX = points(inner.path)[1][0];
        const outerX = points(outer.path)[1][0];
        expect(outerX - innerX).toBe(LANE_SPACING);
    });

    it('caps lane offsets at MAX_LANE', () => {
        const capped = routeArrow(endpoints({ exitLane: MAX_LANE + 5 }));
        const max = routeArrow(endpoints({ exitLane: MAX_LANE }));
        expect(points(capped.path)[1][0]).toBe(points(max.path)[1][0]);
    });

    it('routes same-side (start-to-start) through a shared left anchor', () => {
        const { path } = routeArrow(
            endpoints({
                sourceX: 100,
                targetX: 150,
                exitDirection: 'left',
                enterDirection: 'left'
            })
        );
        const pts = points(path);
        const anchorX = 100 - HORIZONTAL_OFFSET;
        expect(pts[1][0]).toBe(anchorX);
        expect(pts[2]).toEqual([anchorX, 96]);
        expect(pts[3]).toEqual([150, 96]);
    });

    it('detours above the row for same-row relations', () => {
        const { path, midY } = routeArrow(
            endpoints({ targetY: 32, targetRowIndex: 0, targetX: 300 })
        );
        const pts = points(path);
        expect(Math.min(...pts.map(p => p[1]))).toBeLessThan(32);
        expect(midY).toBeLessThan(32);
    });

    it('returns a midpoint lying on the path for the boundary route', () => {
        const { path, midX, midY } = routeArrow(endpoints({ targetX: 100 }));
        const pts = points(path);
        // Midpoint sits on the boundary segment between its two x-extremes
        expect(midY).toBe(ROW_HEIGHT);
        const xs = [pts[2][0], pts[3][0]].sort((a, b) => a - b);
        expect(midX).toBeGreaterThanOrEqual(xs[0]);
        expect(midX).toBeLessThanOrEqual(xs[1]);
    });
});

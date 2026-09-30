import { DiffExpandDirection } from 'src/app/project/model/git-integration.model';
import { DiffHunk } from './diff-parser';

export const EXPAND_CHUNK = 20;

export interface ExpanderTarget {
    hunkIndex: number;
    direction: DiffExpandDirection;
    gap: number;
    singleButton: boolean;
}

export abstract class DiffExpander {
    public static list(
        hunks: DiffHunk[],
        fileLineCount: number,
        useOldSide: boolean
    ): ExpanderTarget[] {
        const result: ExpanderTarget[] = [];
        if (hunks.length === 0) return result;

        const first = hunks[0];
        const firstGap = useOldSide ? first.oldStart - 1 : first.newStart - 1;
        if (firstGap > 0) {
            result.push({
                hunkIndex: 0,
                direction: DiffExpandDirection.Up,
                gap: firstGap,
                singleButton: firstGap <= EXPAND_CHUNK
            });
        }

        for (let i = 0; i < hunks.length - 1; i++) {
            const a = hunks[i];
            const b = hunks[i + 1];
            const gap = useOldSide
                ? b.oldStart - (a.oldStart + a.oldLines)
                : b.newStart - (a.newStart + a.newLines);
            if (gap <= 0) continue;
            if (gap <= EXPAND_CHUNK) {
                result.push({
                    hunkIndex: i,
                    direction: DiffExpandDirection.Down,
                    gap,
                    singleButton: true
                });
            } else {
                result.push({
                    hunkIndex: i,
                    direction: DiffExpandDirection.Down,
                    gap,
                    singleButton: false
                });
                result.push({
                    hunkIndex: i + 1,
                    direction: DiffExpandDirection.Up,
                    gap,
                    singleButton: false
                });
            }
        }

        const last = hunks[hunks.length - 1];
        const lastVisible = useOldSide
            ? last.oldStart + last.oldLines
            : last.newStart + last.newLines;
        const remaining = fileLineCount > 0 ? fileLineCount - lastVisible + 1 : EXPAND_CHUNK;
        if (fileLineCount === 0 || lastVisible <= fileLineCount) {
            result.push({
                hunkIndex: hunks.length - 1,
                direction: DiffExpandDirection.Down,
                gap: Math.max(remaining, 0),
                singleButton: remaining <= EXPAND_CHUNK
            });
        }

        return result;
    }

    public static chunkSize(gap: number): number {
        return gap <= EXPAND_CHUNK ? gap : EXPAND_CHUNK;
    }

    public static startLine(
        hunk: DiffHunk,
        direction: DiffExpandDirection,
        count: number,
        useOldSide: boolean
    ): number {
        if (direction === DiffExpandDirection.Up) {
            const start = useOldSide ? hunk.oldStart : hunk.newStart;
            return start - count - 1;
        }
        const after = useOldSide ? hunk.oldStart + hunk.oldLines : hunk.newStart + hunk.newLines;
        return after - 1;
    }
}

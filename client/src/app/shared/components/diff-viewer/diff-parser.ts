export enum DiffLineKind {
    Context = ' ',
    Add = '+',
    Remove = '-',
    NoNewline = '\\'
}

export interface DiffLine {
    kind: DiffLineKind;
    text: string;
}

export interface DiffHunk {
    oldStart: number;
    oldLines: number;
    newStart: number;
    newLines: number;
    section: string;
    lines: DiffLine[];
}

export enum ExpandDirection {
    Up = 'up',
    Down = 'down'
}

export interface ExpandAction {
    direction: ExpandDirection;
    hunkIndex: number;
    count: number;
}

export interface ExpandSlot {
    beforeHunk: number;
    gap: number | null;
    actions: ExpandAction[];
}

export const EXPAND_STEP = 20;

const DEFAULT_CONTEXT_LINES = 3;

export abstract class DiffParser {
    private static readonly HUNK_HEADER = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(.*)$/;

    public static parse(patch: string): DiffHunk[] {
        const hunks: DiffHunk[] = [];
        let current: DiffHunk | null = null;
        let oldRemaining = 0;
        let newRemaining = 0;

        for (const rawLine of patch.split('\n')) {
            const line = rawLine.replace(/\r$/, '');
            const header = this.HUNK_HEADER.exec(line);
            if (header) {
                current = {
                    oldStart: Number(header[1]),
                    oldLines: header[2] === undefined ? 1 : Number(header[2]),
                    newStart: Number(header[3]),
                    newLines: header[4] === undefined ? 1 : Number(header[4]),
                    section: header[5],
                    lines: []
                };
                oldRemaining = current.oldLines;
                newRemaining = current.newLines;
                hunks.push(current);
                continue;
            }
            if (!current) continue;

            if (line.startsWith(DiffLineKind.NoNewline)) {
                current.lines.push({ kind: DiffLineKind.NoNewline, text: line.slice(1) });
            } else if (line.startsWith(DiffLineKind.Add) && newRemaining > 0) {
                current.lines.push({ kind: DiffLineKind.Add, text: line.slice(1) });
                newRemaining--;
            } else if (line.startsWith(DiffLineKind.Remove) && oldRemaining > 0) {
                current.lines.push({ kind: DiffLineKind.Remove, text: line.slice(1) });
                oldRemaining--;
            } else if (oldRemaining > 0 && newRemaining > 0) {
                current.lines.push({ kind: DiffLineKind.Context, text: line.slice(1) });
                oldRemaining--;
                newRemaining--;
            }
        }
        return hunks;
    }

    public static serialize(hunks: DiffHunk[]): string {
        const out: string[] = [];
        for (const hunk of hunks) {
            out.push(
                `@@ -${hunk.oldStart},${hunk.oldLines} +${hunk.newStart},${hunk.newLines} @@${hunk.section}`
            );
            for (const line of hunk.lines) out.push(line.kind + line.text);
        }
        return out.join('\n');
    }

    public static isExpandable(hunks: DiffHunk[]): boolean {
        if (hunks.length === 0) return false;
        const isAddedFile = hunks[0].oldStart === 0 && hunks[0].oldLines === 0;
        const isDeletedFile = hunks[0].newStart === 0 && hunks[0].newLines === 0;
        return !isAddedFile && !isDeletedFile;
    }

    public static slots(hunks: DiffHunk[], fileLineCount: number | null): ExpandSlot[] {
        if (!this.isExpandable(hunks)) return [];

        const slots: ExpandSlot[] = [];
        hunks.forEach((hunk, idx) => {
            const gap =
                this.firstNewLine(hunk) - (idx === 0 ? 1 : this.nextNewLine(hunks[idx - 1]));
            if (gap <= 0) return;
            const actions: ExpandAction[] = [];
            if (gap <= EXPAND_STEP) {
                actions.push({ direction: ExpandDirection.Up, hunkIndex: idx, count: gap });
            } else {
                if (idx > 0) {
                    actions.push({
                        direction: ExpandDirection.Down,
                        hunkIndex: idx - 1,
                        count: EXPAND_STEP
                    });
                }
                actions.push({ direction: ExpandDirection.Up, hunkIndex: idx, count: EXPAND_STEP });
            }
            slots.push({ beforeHunk: idx, gap, actions });
        });

        const lastIndex = hunks.length - 1;
        const trailingGap =
            fileLineCount === null ? null : fileLineCount - this.nextNewLine(hunks[lastIndex]) + 1;
        const hasMoreBelow = trailingGap === null ? !this.reachesEndOfFile(hunks) : trailingGap > 0;
        if (hasMoreBelow) {
            slots.push({
                beforeHunk: hunks.length,
                gap: trailingGap,
                actions: [
                    {
                        direction: ExpandDirection.Down,
                        hunkIndex: lastIndex,
                        count: Math.min(trailingGap ?? EXPAND_STEP, EXPAND_STEP)
                    }
                ]
            });
        }
        return slots;
    }

    public static expand(hunks: DiffHunk[], fileLines: string[], action: ExpandAction): DiffHunk[] {
        const result = hunks.map(hunk => ({ ...hunk, lines: [...hunk.lines] }));
        const hunk = result[action.hunkIndex];
        if (!hunk) return result;

        if (action.direction === ExpandDirection.Up) {
            const floor =
                action.hunkIndex === 0 ? 1 : this.nextNewLine(result[action.hunkIndex - 1]);
            const first = this.firstNewLine(hunk);
            const count = Math.min(action.count, first - floor);
            const added = this.contextLines(fileLines, first - count, count);
            if (added.length === 0) return hunks;
            const oldFirst = this.firstOldLine(hunk);
            hunk.lines.unshift(...added);
            hunk.newStart = first - added.length;
            hunk.oldStart = oldFirst - added.length;
        } else {
            const next = result[action.hunkIndex + 1];
            const ceiling = next ? this.firstNewLine(next) : fileLines.length + 1;
            const start = this.nextNewLine(hunk);
            const count = Math.min(action.count, ceiling - start);
            const added = this.contextLines(fileLines, start, count);
            if (added.length === 0) return hunks;
            hunk.newStart = this.firstNewLine(hunk);
            hunk.oldStart = this.firstOldLine(hunk);
            hunk.lines.push(...added);
        }
        const growth = hunk.lines.length - hunks[action.hunkIndex].lines.length;
        hunk.newLines += growth;
        hunk.oldLines += growth;

        return this.mergeAdjacent(result);
    }

    private static mergeAdjacent(hunks: DiffHunk[]): DiffHunk[] {
        const merged: DiffHunk[] = [];
        for (const hunk of hunks) {
            const previous = merged[merged.length - 1];
            if (previous && this.nextNewLine(previous) === this.firstNewLine(hunk)) {
                previous.newStart = this.firstNewLine(previous);
                previous.oldStart = this.firstOldLine(previous);
                previous.lines.push(...hunk.lines);
                previous.oldLines += hunk.oldLines;
                previous.newLines += hunk.newLines;
            } else {
                merged.push(hunk);
            }
        }
        return merged;
    }

    private static reachesEndOfFile(hunks: DiffHunk[]): boolean {
        const lastLines = hunks[hunks.length - 1].lines;
        if (lastLines.at(-1)?.kind === DiffLineKind.NoNewline) return true;
        return this.countContext([...lastLines].reverse()) < this.contextSize(hunks[0]);
    }

    private static contextSize(first: DiffHunk): number {
        if (this.firstNewLine(first) === 1) return DEFAULT_CONTEXT_LINES;
        return this.countContext(first.lines);
    }

    private static countContext(lines: DiffLine[]): number {
        const end = lines.findIndex(line => line.kind !== DiffLineKind.Context);
        return end === -1 ? lines.length : end;
    }

    private static contextLines(fileLines: string[], firstLine: number, count: number): DiffLine[] {
        if (count <= 0) return [];
        return fileLines
            .slice(firstLine - 1, firstLine - 1 + count)
            .map(text => ({ kind: DiffLineKind.Context, text }));
    }

    private static firstNewLine(hunk: DiffHunk): number {
        return hunk.newLines > 0 ? hunk.newStart : hunk.newStart + 1;
    }

    private static nextNewLine(hunk: DiffHunk): number {
        return this.firstNewLine(hunk) + hunk.newLines;
    }

    private static firstOldLine(hunk: DiffHunk): number {
        return hunk.oldLines > 0 ? hunk.oldStart : hunk.oldStart + 1;
    }
}

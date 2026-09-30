import { DiffExpandDirection } from 'src/app/project/model/git-integration.model';

export enum DiffLineKind {
    Context = 'context',
    Add = 'add',
    Remove = 'remove'
}

export interface DiffLine {
    kind: DiffLineKind;
    oldLine: number | null;
    newLine: number | null;
    text: string;
}

export interface DiffHunk {
    oldStart: number;
    oldLines: number;
    newStart: number;
    newLines: number;
    lines: DiffLine[];
}

export interface ParsedDiffFile {
    oldPath: string;
    newPath: string;
    isDeleted: boolean;
    hunks: DiffHunk[];
}

export abstract class DiffParser {
    private static readonly HUNK_RE = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@/;

    public static parseFile(
        oldPath: string,
        newPath: string,
        patch: string,
        isDeleted: boolean
    ): ParsedDiffFile {
        const hunks: DiffHunk[] = [];
        let current: DiffHunk | null = null;
        let oldLine = 0;
        let newLine = 0;

        for (const raw of patch.split('\n')) {
            const line = raw.replace(/\r$/, '');
            const match = this.HUNK_RE.exec(line);
            if (match) {
                current = {
                    oldStart: Math.max(1, parseInt(match[1], 10)),
                    oldLines: match[2] ? parseInt(match[2], 10) : 1,
                    newStart: Math.max(1, parseInt(match[3], 10)),
                    newLines: match[4] ? parseInt(match[4], 10) : 1,
                    lines: []
                };
                oldLine = current.oldStart;
                newLine = current.newStart;
                hunks.push(current);
                continue;
            }
            if (!current) continue;

            if (line.startsWith('+') && !line.startsWith('+++')) {
                current.lines.push({
                    kind: DiffLineKind.Add,
                    oldLine: null,
                    newLine: newLine++,
                    text: line.slice(1)
                });
            } else if (line.startsWith('-') && !line.startsWith('---')) {
                current.lines.push({
                    kind: DiffLineKind.Remove,
                    oldLine: oldLine++,
                    newLine: null,
                    text: line.slice(1)
                });
            } else {
                const text = line.startsWith(' ') ? line.slice(1) : line;
                current.lines.push({ kind: DiffLineKind.Context, oldLine, newLine, text });
                oldLine++;
                newLine++;
            }
        }
        return { oldPath, newPath, isDeleted, hunks };
    }

    public static serializeFile(file: ParsedDiffFile): string {
        const out: string[] = [];
        out.push(`--- a/${file.oldPath}`);
        out.push(`+++ b/${file.newPath}`);
        for (const h of file.hunks) {
            out.push(`@@ -${h.oldStart},${h.oldLines} +${h.newStart},${h.newLines} @@`);
            for (const ln of h.lines) {
                switch (ln.kind) {
                    case DiffLineKind.Context:
                        out.push(` ${ln.text}`);
                        break;
                    case DiffLineKind.Add:
                        out.push(`+${ln.text}`);
                        break;
                    case DiffLineKind.Remove:
                        out.push(`-${ln.text}`);
                        break;
                }
            }
        }
        return out.join('\n');
    }

    public static expandContext(
        file: ParsedDiffFile,
        hunkIndex: number,
        direction: DiffExpandDirection,
        lines: string[]
    ): void {
        const hunk = file.hunks[hunkIndex];
        if (!hunk || lines.length === 0) return;

        if (direction === DiffExpandDirection.Up) {
            const prepend = lines.map((text, i) => ({
                kind: DiffLineKind.Context,
                oldLine: hunk.oldStart - lines.length + i,
                newLine: hunk.newStart - lines.length + i,
                text
            }));
            hunk.lines.unshift(...prepend);
            hunk.oldStart -= lines.length;
            hunk.newStart -= lines.length;
            hunk.oldLines += lines.length;
            hunk.newLines += lines.length;
        } else {
            const oldAfter = hunk.oldStart + hunk.oldLines;
            const newAfter = hunk.newStart + hunk.newLines;
            const append = lines.map((text, i) => ({
                kind: DiffLineKind.Context,
                oldLine: oldAfter + i,
                newLine: newAfter + i,
                text
            }));
            hunk.lines.push(...append);
            hunk.oldLines += lines.length;
            hunk.newLines += lines.length;
        }
    }

    public static mergeHunksIfAdjacent(file: ParsedDiffFile, indexA: number, indexB: number): void {
        const a = file.hunks[indexA];
        const b = file.hunks[indexB];
        if (!a || !b) return;
        const oldGap = b.oldStart - (a.oldStart + a.oldLines);
        const newGap = b.newStart - (a.newStart + a.newLines);
        if (oldGap === 0 && newGap === 0) {
            a.lines.push(...b.lines);
            a.oldLines += b.oldLines;
            a.newLines += b.newLines;
            file.hunks.splice(indexB, 1);
        }
    }
}

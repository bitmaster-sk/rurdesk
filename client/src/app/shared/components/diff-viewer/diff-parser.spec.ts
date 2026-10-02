import { describe, it, expect } from 'vitest';
import { DiffLineKind, DiffParser, EXPAND_STEP, ExpandDirection } from './diff-parser';

const FILE = Array.from({ length: 60 }, (_, idx) => `line ${idx + 1}`);

const TWO_HUNKS = [
    '@@ -5,3 +5,3 @@ func a',
    ' line 5',
    '-old 6',
    '+line 6',
    ' line 7',
    '@@ -40,2 +40,2 @@ func b',
    '-old 40',
    '+line 40',
    ' line 41',
    ''
].join('\n');

function visibleNewLines(patch: string): number[] {
    const numbers: number[] = [];
    for (const hunk of DiffParser.parse(patch)) {
        let lineNumber = hunk.newStart;
        for (const line of hunk.lines) {
            if (line.kind === DiffLineKind.Context || line.kind === DiffLineKind.Add) {
                numbers.push(lineNumber++);
            }
        }
    }
    return numbers;
}

describe('DiffParser.parse', () => {
    it('reads hunk ranges, the section heading and every line kind', () => {
        const hunks = DiffParser.parse(TWO_HUNKS);
        expect(hunks).toHaveLength(2);
        expect(hunks[0]).toMatchObject({ oldStart: 5, oldLines: 3, newStart: 5, newLines: 3 });
        expect(hunks[0].section).toBe(' func a');
        expect(hunks[0].lines.map(line => line.kind)).toEqual([
            DiffLineKind.Context,
            DiffLineKind.Remove,
            DiffLineKind.Add,
            DiffLineKind.Context
        ]);
    });

    it('ignores the trailing newline of the patch', () => {
        const hunks = DiffParser.parse(TWO_HUNKS);
        expect(hunks[1].lines).toHaveLength(3);
    });

    it('treats an empty line inside a hunk as an empty context line', () => {
        const hunks = DiffParser.parse('@@ -1,3 +1,3 @@\n a\n\n-b\n+c');
        expect(hunks[0].lines[1]).toEqual({ kind: DiffLineKind.Context, text: '' });
    });

    it('keeps the no-newline marker without counting it as a line', () => {
        const patch =
            '@@ -1 +1 @@\n-a\n\\ No newline at end of file\n+b\n\\ No newline at end of file';
        const hunks = DiffParser.parse(patch);
        expect(hunks[0].lines.filter(line => line.kind === DiffLineKind.NoNewline)).toHaveLength(2);
        expect(DiffParser.serialize(hunks)).toBe(
            '@@ -1,1 +1,1 @@\n-a\n\\ No newline at end of file\n+b\n\\ No newline at end of file'
        );
    });
});

describe('DiffParser.slots', () => {
    it('offers nothing for an added or a deleted file', () => {
        expect(DiffParser.slots(DiffParser.parse('@@ -0,0 +1,2 @@\n+a\n+b'), null)).toEqual([]);
        expect(DiffParser.slots(DiffParser.parse('@@ -1,2 +0,0 @@\n-a\n-b'), null)).toEqual([]);
    });

    it('offers a single expand-all above the first hunk when the gap is small', () => {
        const [first] = DiffParser.slots(DiffParser.parse(TWO_HUNKS), null);
        expect(first.beforeHunk).toBe(0);
        expect(first.gap).toBe(4);
        expect(first.actions).toEqual([{ direction: ExpandDirection.Up, hunkIndex: 0, count: 4 }]);
    });

    it('offers a step down and a step up for a large gap between hunks', () => {
        const between = DiffParser.slots(DiffParser.parse(TWO_HUNKS), null)[1];
        expect(between.gap).toBe(32);
        expect(between.actions).toEqual([
            { direction: ExpandDirection.Down, hunkIndex: 0, count: EXPAND_STEP },
            { direction: ExpandDirection.Up, hunkIndex: 1, count: EXPAND_STEP }
        ]);
    });

    it('offers nothing above a hunk that starts on line 1', () => {
        const slots = DiffParser.slots(
            DiffParser.parse('@@ -1,4 +1,4 @@\n-a\n+b\n c\n d\n e'),
            null
        );
        expect(slots.map(slot => slot.beforeHunk)).toEqual([1]);
    });

    it('offers a trailing step while the file length is unknown and hides it at the end of file', () => {
        const hunks = DiffParser.parse(TWO_HUNKS);
        expect(DiffParser.slots(hunks, null).at(-1)?.gap).toBeNull();
        expect(DiffParser.slots(hunks, 45).at(-1)?.gap).toBe(4);
        expect(DiffParser.slots(hunks, 41).map(slot => slot.beforeHunk)).toEqual([0, 1]);
    });

    it('offers no trailing step when the last hunk has less context than the diff uses', () => {
        const atEnd = '@@ -5,5 +5,5 @@\n a\n b\n c\n-d\n+e\n f';
        const beforeEnd = '@@ -5,7 +5,7 @@\n a\n b\n c\n-d\n+e\n f\n g\n h';
        expect(DiffParser.slots(DiffParser.parse(atEnd), null).map(s => s.beforeHunk)).toEqual([0]);
        expect(DiffParser.slots(DiffParser.parse(beforeEnd), null).map(s => s.beforeHunk)).toEqual([
            0, 1
        ]);
    });

    it('offers no trailing step when the patch ends with the no-newline marker', () => {
        const patch = '@@ -5,4 +5,4 @@\n a\n b\n c\n-d\n+e\n\\ No newline at end of file';
        expect(DiffParser.slots(DiffParser.parse(patch), null).map(s => s.beforeHunk)).toEqual([0]);
    });
});

describe('DiffParser.expand', () => {
    it('reveals the lines above the first hunk', () => {
        const hunks = DiffParser.expand(DiffParser.parse(TWO_HUNKS), FILE, {
            direction: ExpandDirection.Up,
            hunkIndex: 0,
            count: 4
        });
        const patch = DiffParser.serialize(hunks);
        expect(patch.startsWith('@@ -1,7 +1,7 @@ func a\n line 1\n line 2')).toBe(true);
        expect(visibleNewLines(patch).slice(0, 5)).toEqual([1, 2, 3, 4, 5]);
    });

    it('reveals a step of lines below a hunk', () => {
        const hunks = DiffParser.expand(DiffParser.parse(TWO_HUNKS), FILE, {
            direction: ExpandDirection.Down,
            hunkIndex: 0,
            count: EXPAND_STEP
        });
        expect(hunks).toHaveLength(2);
        expect(hunks[0]).toMatchObject({ newStart: 5, newLines: 23, oldLines: 23 });
        expect(hunks[0].lines.at(-1)?.text).toBe('line 27');
    });

    it('merges two hunks once the gap between them is filled', () => {
        let hunks = DiffParser.parse(TWO_HUNKS);
        hunks = DiffParser.expand(hunks, FILE, {
            direction: ExpandDirection.Down,
            hunkIndex: 0,
            count: EXPAND_STEP
        });
        hunks = DiffParser.expand(hunks, FILE, {
            direction: ExpandDirection.Up,
            hunkIndex: 1,
            count: EXPAND_STEP
        });
        expect(hunks).toHaveLength(1);
        expect(visibleNewLines(DiffParser.serialize(hunks))).toEqual(
            Array.from({ length: 37 }, (_, idx) => idx + 5)
        );
    });

    it('never reveals lines past the end of the file', () => {
        const hunks = DiffParser.expand(DiffParser.parse(TWO_HUNKS), FILE.slice(0, 43), {
            direction: ExpandDirection.Down,
            hunkIndex: 1,
            count: EXPAND_STEP
        });
        expect(hunks[1].lines.at(-1)?.text).toBe('line 43');
        expect(DiffParser.slots(hunks, 43).map(slot => slot.beforeHunk)).toEqual([0, 1]);
    });

    it('leaves the hunks untouched when there is nothing left to reveal', () => {
        const hunks = DiffParser.parse(TWO_HUNKS);
        const result = DiffParser.expand(hunks, FILE.slice(0, 41), {
            direction: ExpandDirection.Down,
            hunkIndex: 1,
            count: EXPAND_STEP
        });
        expect(result).toBe(hunks);
    });
});

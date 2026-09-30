import { describe, it, expect } from 'vitest';
import { DiffExpandDirection } from 'src/app/project/model/git-integration.model';
import { DiffLineKind, DiffParser } from './diff-parser';

const PATCH = [
    '@@ -10,6 +10,6 @@ package main',
    ' import "fmt"',
    ' import "os"',
    ' ',
    '-func old() {}',
    '+func new() {}',
    ' ',
    ' func main() {}'
].join('\n');

describe('DiffParser.parseFile', () => {
    it('parses a single hunk with correct line numbers', () => {
        const file = DiffParser.parseFile('main.go', 'main.go', PATCH, false);
        expect(file.hunks).toHaveLength(1);
        const h = file.hunks[0];
        expect(h.oldStart).toBe(10);
        expect(h.newStart).toBe(10);
        expect(h.lines.filter(l => l.kind === DiffLineKind.Context)).toHaveLength(5);
        expect(h.lines.filter(l => l.kind === DiffLineKind.Add)).toHaveLength(1);
        expect(h.lines.filter(l => l.kind === DiffLineKind.Remove)).toHaveLength(1);
    });

    it('strips the leading space from context lines', () => {
        const file = DiffParser.parseFile('main.go', 'main.go', PATCH, false);
        const ctx = file.hunks[0].lines.find(l => l.kind === DiffLineKind.Context);
        expect(ctx?.text).toBe('import "fmt"');
    });

    it('marks deleted files', () => {
        const file = DiffParser.parseFile(
            'gone.go',
            'gone.go',
            '@@ -1,1 +0,0 @@\n-removed\n',
            true
        );
        expect(file.isDeleted).toBe(true);
    });
});

describe('DiffParser.expandContext', () => {
    it('inserts context lines above the first hunk and shifts the hunk', () => {
        const file = DiffParser.parseFile('main.go', 'main.go', PATCH, false);
        DiffParser.expandContext(file, 0, DiffExpandDirection.Up, [
            'package main',
            '',
            'import "flag"'
        ]);
        expect(file.hunks[0].oldStart).toBe(7);
        expect(file.hunks[0].newStart).toBe(7);
        expect(file.hunks[0].lines[0]).toEqual({
            kind: DiffLineKind.Context,
            oldLine: 7,
            newLine: 7,
            text: 'package main'
        });
    });

    it('appends context lines below the hunk', () => {
        const file = DiffParser.parseFile('main.go', 'main.go', PATCH, false);
        DiffParser.expandContext(file, 0, DiffExpandDirection.Down, ['// trailing']);
        const last = file.hunks[0].lines[file.hunks[0].lines.length - 1];
        expect(last).toEqual({
            kind: DiffLineKind.Context,
            oldLine: 16,
            newLine: 16,
            text: '// trailing'
        });
    });
});

describe('DiffParser.mergeHunksIfAdjacent', () => {
    it('fuses two hunks when the gap is fully closed', () => {
        const file = DiffParser.parseFile('main.go', 'main.go', PATCH, false);
        // Place a second hunk directly after the first: hunk1 covers old 10..15,
        // so hunk2 starts at old line 16.
        const second = '@@ -16,2 +16,2 @@\n context-15\n-changed\n+changed-x\n';
        file.hunks.push(DiffParser.parseFile('main.go', 'main.go', second, false).hunks[0]);
        // No gap left; merge should fuse the two hunks immediately.
        DiffParser.mergeHunksIfAdjacent(file, 0, 1);
        expect(file.hunks).toHaveLength(1);
        expect(file.hunks[0].oldLines).toBe(8);
    });
});

describe('DiffParser.serializeFile', () => {
    it('round-trips through serialization', () => {
        const file = DiffParser.parseFile('main.go', 'main.go', PATCH, false);
        DiffParser.expandContext(file, 0, DiffExpandDirection.Down, ['// trailing']);
        const out = DiffParser.serializeFile(file);
        expect(out).toContain('@@ -10,7 +10,7 @@');
        expect(out).toContain(' // trailing');
    });
});

import { describe, it, expect } from 'vitest';
import { DiffExpandDirection } from 'src/app/project/model/git-integration.model';
import { DiffExpander } from './diff-expander';
import { DiffParser } from './diff-parser';

const PATCH = [
    '@@ -2,2 +2,2 @@ a',
    ' 2',
    '-3',
    '+3x',
    '@@ -12,2 +12,2 @@ b',
    ' 12',
    '-13',
    '+13x'
].join('\n');

describe('DiffExpander.list', () => {
    it('creates an up expander before the first hunk', () => {
        const file = DiffParser.parseFile('f.txt', 'f.txt', PATCH, false);
        const targets = DiffExpander.list(file.hunks, 0, false);
        const up = targets.find(t => t.direction === DiffExpandDirection.Up && t.hunkIndex === 0);
        expect(up?.gap).toBe(1);
        expect(up?.singleButton).toBe(true);
    });

    it('creates a single expand-all button when the between-hunks gap is <= 20', () => {
        const file = DiffParser.parseFile('f.txt', 'f.txt', PATCH, false);
        const targets = DiffExpander.list(file.hunks, 0, false);
        const between = targets.filter(
            t => t.hunkIndex === 0 && t.direction === DiffExpandDirection.Down
        );
        expect(between).toHaveLength(1);
        expect(between[0].gap).toBe(8);
        expect(between[0].singleButton).toBe(true);
    });

    it('creates separate up and down expanders for large gaps', () => {
        const patch = [
            '@@ -2,2 +2,2 @@ a',
            ' 2',
            '-3',
            '+3x',
            '@@ -30,2 +30,2 @@ b',
            ' 30',
            '-31',
            '+31x'
        ].join('\n');
        const file = DiffParser.parseFile('f.txt', 'f.txt', patch, false);
        const targets = DiffExpander.list(file.hunks, 0, false);
        const down = targets.find(
            t => t.direction === DiffExpandDirection.Down && t.hunkIndex === 0
        );
        const up = targets.find(t => t.direction === DiffExpandDirection.Up && t.hunkIndex === 1);
        expect(down?.singleButton).toBe(false);
        expect(up?.singleButton).toBe(false);
    });

    it('hides the down expander when the known line count reaches EOF', () => {
        const file = DiffParser.parseFile('f.txt', 'f.txt', PATCH, false);
        const targets = DiffExpander.list(file.hunks, 13, false);
        const down = targets.find(
            t => t.direction === DiffExpandDirection.Down && t.hunkIndex === 1
        );
        expect(down).toBeUndefined();
    });

    it('uses the old side for deleted files', () => {
        const file = DiffParser.parseFile('f.txt', 'f.txt', PATCH, true);
        const targets = DiffExpander.list(file.hunks, 0, true);
        const up = targets.find(t => t.direction === DiffExpandDirection.Up && t.hunkIndex === 0);
        expect(up?.gap).toBe(1);
    });
});

describe('DiffExpander.startLine', () => {
    it('computes a 0-based start index above a hunk', () => {
        const file = DiffParser.parseFile('f.txt', 'f.txt', PATCH, false);
        const hunk = file.hunks[0];
        expect(DiffExpander.startLine(hunk, DiffExpandDirection.Up, 1, false)).toBe(0);
    });

    it('computes a 0-based start index below a hunk', () => {
        const file = DiffParser.parseFile('f.txt', 'f.txt', PATCH, false);
        const hunk = file.hunks[0];
        expect(DiffExpander.startLine(hunk, DiffExpandDirection.Down, 2, false)).toBe(3);
    });
});

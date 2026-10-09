import { WikiMergeChunkKind } from '../constants/wiki-merge-chunk-kind.enum';
import { WikiConflictChoice, WikiConflictPick } from '../entity/wiki-conflict-choice.entity';
import { WikiMergeChunk } from '../model/wiki-save.model';
import { WikiMergeConverter } from './wiki-merge.converter';

const chunks: WikiMergeChunk[] = [
    { kind: WikiMergeChunkKind.Stable, lines: ['intro'] },
    { kind: WikiMergeChunkKind.Conflict, base: ['old'], mine: ['mine'], theirs: ['theirs'] },
    { kind: WikiMergeChunkKind.Stable, lines: ['outro'] }
];

const choose = (pick: WikiConflictPick, custom = ''): Map<number, WikiConflictChoice> =>
    new Map([[1, { pick, custom }]]);

describe('WikiMergeConverter', () => {
    it('is unresolved until every conflict has a choice', () => {
        expect(WikiMergeConverter.isResolved(chunks, new Map())).toBe(false);
        expect(WikiMergeConverter.isResolved(chunks, choose(WikiConflictPick.Mine))).toBe(true);
    });

    it('composes the text from the chosen side', () => {
        expect(WikiMergeConverter.toText(chunks, choose(WikiConflictPick.Mine))).toBe(
            'intro\nmine\noutro'
        );
        expect(WikiMergeConverter.toText(chunks, choose(WikiConflictPick.Theirs))).toBe(
            'intro\ntheirs\noutro'
        );
        expect(WikiMergeConverter.toText(chunks, choose(WikiConflictPick.Both))).toBe(
            'intro\nmine\ntheirs\noutro'
        );
        expect(WikiMergeConverter.toText(chunks, choose(WikiConflictPick.Custom, 'a\nb'))).toBe(
            'intro\na\nb\noutro'
        );
    });

    it('lists the indexes of conflict chunks', () => {
        expect(WikiMergeConverter.conflictIndexes(chunks)).toEqual([1]);
    });
});

describe('WikiMergeConverter.toConflict', () => {
    it('reads the merge out of a 409 that carries one', () => {
        const conflict = { merge: { chunks, conflicts: 1 } };

        expect(WikiMergeConverter.toConflict({ status: 409, error: conflict })).toBe(conflict);
    });

    it('ignores other errors, including a 409 without a merge', () => {
        expect(
            WikiMergeConverter.toConflict({ status: 409, error: { code: 'WIKI_SLUG_TAKEN' } })
        ).toBeNull();
        expect(WikiMergeConverter.toConflict({ status: 500, error: { merge: {} } })).toBeNull();
        expect(WikiMergeConverter.toConflict(null)).toBeNull();
    });
});

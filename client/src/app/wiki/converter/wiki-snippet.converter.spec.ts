import { WikiSnippetConverter } from './wiki-snippet.converter';

describe('WikiSnippetConverter', () => {
    it('splits the server snippet into plain and highlighted parts', () => {
        expect(WikiSnippetConverter.toParts('the <<worktree>> is kept <<7>> days')).toEqual([
            { text: 'the ', isMatch: false },
            { text: 'worktree', isMatch: true },
            { text: ' is kept ', isMatch: false },
            { text: '7', isMatch: true },
            { text: ' days', isMatch: false }
        ]);
    });

    it('returns plain text when nothing matched', () => {
        expect(WikiSnippetConverter.toParts('plain')).toEqual([{ text: 'plain', isMatch: false }]);
        expect(WikiSnippetConverter.toParts('')).toEqual([]);
    });
});

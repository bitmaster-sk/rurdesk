import { WikiAnchorConverter } from './wiki-anchor.converter';

describe('WikiAnchorConverter', () => {
    it('makes anchors from heading text the same way as page slugs', () => {
        expect(WikiAnchorConverter.toAnchors(['Rollback plán', 'Ako nasadzujeme?'])).toEqual([
            'rollback-plan',
            'ako-nasadzujeme'
        ]);
    });

    it('numbers repeated headings and names empty ones', () => {
        expect(WikiAnchorConverter.toAnchors(['Kroky', 'Kroky', '!!!', 'Kroky'])).toEqual([
            'kroky',
            'kroky-1',
            'section',
            'kroky-2'
        ]);
    });

    it('lists headings of a page for completion and skips code blocks', () => {
        const markdown =
            '# Release\ntext\n## Kroky ##\n```\n# not a heading\n```\n#hashtag\n### Rollback';
        expect(WikiAnchorConverter.toHeadings(markdown)).toEqual(['Release', 'Kroky', 'Rollback']);
    });
});

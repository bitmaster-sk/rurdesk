import { WikiCalloutKind } from '../constants/wiki-callout-kind.enum';
import { WikiCalloutConverter } from './wiki-callout.converter';

const labels = WikiCalloutConverter.toLabels(key => key.split('.').at(-1)!);

describe('WikiCalloutConverter', () => {
    it('turns a GitHub alert into a titled block whose content stays Markdown', () => {
        expect(
            WikiCalloutConverter.toHtml('> [!WARNING]\n> Run **migrations** first.\nAfter', labels)
        ).toBe(
            [
                '<div class="wiki-callout wiki-callout--warning">',
                '<div class="wiki-callout__title">WARNING</div>',
                '',
                'Run **migrations** first.',
                '',
                '</div>',
                '',
                'After'
            ].join('\n')
        );
    });

    it('accepts any letter case and every GitHub kind', () => {
        const html = WikiCalloutConverter.toHtml('> [!note]\n> a', labels);
        expect(html).toContain('wiki-callout--note');
        expect(Object.keys(WikiCalloutConverter.labelKeys)).toEqual(Object.values(WikiCalloutKind));
    });

    it('leaves plain quotes and code samples alone', () => {
        const body = '> just a quote\n```\n> [!CAUTION]\n```';
        expect(WikiCalloutConverter.toHtml(body, labels)).toBe(body);
    });

    it('escapes the title', () => {
        const html = WikiCalloutConverter.toHtml('> [!TIP]\n> x', {
            ...labels,
            [WikiCalloutKind.Tip]: '<b>'
        });
        expect(html).toContain('&lt;b&gt;');
    });
});

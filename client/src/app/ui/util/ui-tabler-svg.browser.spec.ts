import { IconHeartFilled, IconPaperclip } from '@tabler/icons-angular';
import { describe, expect, it } from 'vitest';
import { UiTablerSvg } from './ui-tabler-svg';

describe('UiTablerSvg.create', () => {
    it('draws an outline icon with the stroke in the current text color', () => {
        const svg = UiTablerSvg.create(IconPaperclip, 16, 'chip-icon');

        expect(svg.getAttribute('width')).toBe('16');
        expect(svg.getAttribute('stroke')).toBe('currentColor');
        expect(svg.getAttribute('fill')).toBe('none');
        expect(svg.getAttribute('class')).toBe('chip-icon');
        expect(svg.dataset['icon']).toBe('paperclip');
        expect(svg.querySelector('path')?.getAttribute('d')).toBe(IconPaperclip.nodes[0][1]['d']);
        expect(svg.querySelector('path')?.hasAttribute('key')).toBe(false);
    });

    it('fills a filled icon instead of stroking it', () => {
        const svg = UiTablerSvg.create(IconHeartFilled, 20);

        expect(svg.getAttribute('fill')).toBe('currentColor');
        expect(svg.getAttribute('stroke')).toBe('none');
    });
});

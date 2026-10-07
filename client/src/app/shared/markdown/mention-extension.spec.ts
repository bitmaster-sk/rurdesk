import { describe, it, expect } from 'vitest';
import { Marked } from 'marked';
import { mentionExtension } from './mention-extension';

describe('mentionExtension', () => {
    it('renders @[Jan](user:1) as a .mention-chip span', () => {
        const html = new Marked(mentionExtension).parse('@[Jan](user:1)');
        expect(html).toContain('<span class="mention-chip">@Jan</span>');
    });

    it('does not render a chip for a mention token inside inline code', () => {
        const html = new Marked(mentionExtension).parse('`@[Jan](user:1)`');
        expect(html).not.toContain('mention-chip');
    });
});

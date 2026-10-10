import { describe, expect, it } from 'vitest';
import { AttachmentLink } from './attachment-link';

const idImage = '0b6f1c1e-7a52-4d4b-9a51-2f7c3f0d9e11';
const idFile = '5d2a0c43-1b7e-4f4e-8e0d-6a9b1c2d3e4f';

describe('AttachmentLink.parse', () => {
    it('splits text into plain parts and image and file links', () => {
        const parts = AttachmentLink.parse(
            `a ![shot.png](attachment:${idImage}) b [r.pdf](attachment:${idFile})`
        );

        expect(parts).toEqual([
            { type: 'text', text: 'a ' },
            {
                type: 'attachment',
                markdown: `![shot.png](attachment:${idImage})`,
                name: 'shot.png',
                idAttachment: idImage,
                isImage: true
            },
            { type: 'text', text: ' b ' },
            {
                type: 'attachment',
                markdown: `[r.pdf](attachment:${idFile})`,
                name: 'r.pdf',
                idAttachment: idFile,
                isImage: false
            }
        ]);
    });

    it('leaves ordinary links and malformed ids as text', () => {
        const text = '[site](https://example.com) [x](attachment:not-a-uuid)';

        expect(AttachmentLink.parse(text)).toEqual([{ type: 'text', text }]);
    });
});

describe('AttachmentLink.hasAny', () => {
    it('finds an image or file link', () => {
        expect(AttachmentLink.hasAny(`look ![a.png](attachment:${idImage})`)).toBe(true);
        expect(AttachmentLink.hasAny(`[b.pdf](attachment:${idFile})`)).toBe(true);
    });

    it('ignores text without attachment links', () => {
        expect(AttachmentLink.hasAny('[site](https://example.com) plain text')).toBe(false);
    });
});

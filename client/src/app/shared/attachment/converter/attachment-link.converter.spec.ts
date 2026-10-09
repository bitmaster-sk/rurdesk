import { describe, expect, it } from 'vitest';
import { AttachmentLinkConverter } from './attachment-link.converter';

const id = '123e4567-e89b-12d3-a456-426614174000';

describe('AttachmentLinkConverter', () => {
    it('turns image and file links into marked in-page links', () => {
        const out = AttachmentLinkConverter.toMarkerMarkdown(
            `see ![shot](attachment:${id}) and [doc.pdf](attachment:${id})`
        );
        expect(out).toBe(
            `see [shot](#attachment-${id} "attachment-image") and [doc.pdf](#attachment-${id} "attachment-file")`
        );
    });

    it('leaves attachment links inside code untouched', () => {
        const body = `\`![x](attachment:${id})\``;
        expect(AttachmentLinkConverter.toMarkerMarkdown(body)).toBe(body);
    });

    it('reads the attachment id back from a marked href', () => {
        expect(AttachmentLinkConverter.toIdAttachment(`#attachment-${id}`)).toBe(id);
        expect(AttachmentLinkConverter.toIdAttachment('/project/1')).toBeNull();
    });

    it('rejects a marker that does not carry an attachment id', () => {
        expect(
            AttachmentLinkConverter.toIdAttachment('#attachment-../../admin/user/1/api-key')
        ).toBeNull();
        expect(AttachmentLinkConverter.toIdAttachment('#attachment-')).toBeNull();
        expect(
            AttachmentLinkConverter.toIdAttachment(
                '#attachment-11111111-1111-1111-1111-111111111111/../x'
            )
        ).toBeNull();
    });

    it('formats sizes', () => {
        expect(AttachmentLinkConverter.toSizeLabel(512)).toBe('512 B');
        expect(AttachmentLinkConverter.toSizeLabel(2048)).toBe('2.0 KB');
        expect(AttachmentLinkConverter.toSizeLabel(3 * 1024 * 1024)).toBe('3.0 MB');
    });
});

describe('AttachmentLinkConverter.toUploadedLink', () => {
    const base = { idAttachment: 'id-1', size: 1 };

    it('writes an image link with a leading bang', () => {
        expect(
            AttachmentLinkConverter.toUploadedLink({
                ...base,
                fileName: 'p.png',
                mimeType: 'image/png'
            })
        ).toEqual({ name: 'p.png', markdown: '![p.png](attachment:id-1)' });
    });

    it('writes a plain link for other files and strips brackets from the name', () => {
        expect(
            AttachmentLinkConverter.toUploadedLink({
                ...base,
                fileName: 'a[1].pdf',
                mimeType: 'application/pdf'
            }).markdown
        ).toBe('[a1.pdf](attachment:id-1)');
    });
});

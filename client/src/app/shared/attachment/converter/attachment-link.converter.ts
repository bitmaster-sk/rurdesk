import { MarkdownCode } from 'src/app/shared/markdown/markdown-code';
import { UploadedAttachmentLink } from '../entity/uploaded-attachment-link.entity';
import { UploadedAttachment } from '../model/attachment.model';
import { AttachmentLink } from '../util/attachment-link';

export abstract class AttachmentLinkConverter {
    public static readonly imageTitle = 'attachment-image';
    public static readonly fileTitle = 'attachment-file';
    public static readonly hrefPrefix = '#attachment-';
    private static readonly idPattern =
        /#attachment-([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})$/;

    public static toMarkerMarkdown(body: string): string {
        return MarkdownCode.replaceOutsideCode(body, part => AttachmentLinkConverter.replace(part));
    }

    public static toUploadedLink(attachment: UploadedAttachment): UploadedAttachmentLink {
        const name = attachment.fileName.replace(/[[\]]/g, '');
        const link = `[${name}](attachment:${attachment.idAttachment})`;
        return {
            name: attachment.fileName,
            markdown: attachment.mimeType.startsWith('image/') ? `!${link}` : link
        };
    }

    public static toIdAttachment(href: string): string | null {
        return AttachmentLinkConverter.idPattern.exec(href)?.[1] ?? null;
    }

    public static toSizeLabel(bytes: number): string {
        if (bytes < 1024) {
            return `${bytes} B`;
        }
        if (bytes < 1024 * 1024) {
            return `${(bytes / 1024).toFixed(1)} KB`;
        }
        return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
    }

    private static replace(text: string): string {
        return AttachmentLink.parse(text)
            .map(part =>
                part.type === 'text'
                    ? part.text
                    : `[${part.name}](${AttachmentLinkConverter.hrefPrefix}${part.idAttachment} "${
                          part.isImage
                              ? AttachmentLinkConverter.imageTitle
                              : AttachmentLinkConverter.fileTitle
                      }")`
            )
            .join('');
    }
}

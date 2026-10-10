export type AttachmentLinkPart =
    | { type: 'text'; text: string }
    | {
          type: 'attachment';
          markdown: string;
          name: string;
          idAttachment: string;
          isImage: boolean;
      };

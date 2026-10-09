export type AttachmentState =
    | { status: 'loading' }
    | { status: 'missing' }
    | { status: 'ready'; url: string; mimeType: string; size: number };

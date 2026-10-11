import { describe, expect, it } from 'vitest';
import { ClipboardContent } from './clipboard-content';

function clipboard(entries: Record<string, string>): DataTransfer {
    return {
        types: Object.keys(entries),
        getData: (type: string): string => entries[type] ?? ''
    } as unknown as DataTransfer;
}

describe('ClipboardContent', () => {
    it('treats a copied spreadsheet range as text', () => {
        expect(
            ClipboardContent.isOfficeText(
                clipboard({ 'text/html': '<table></table>', 'text/plain': 'A1\tB1' })
            )
        ).toBe(true);
    });

    it('treats a file copied in Finder as a file even though its name comes along', () => {
        expect(ClipboardContent.isOfficeText(clipboard({ 'text/plain': 'report.pdf' }))).toBe(
            false
        );
    });

    it('treats an image copied from a web page as a file', () => {
        expect(ClipboardContent.isOfficeText(clipboard({ 'text/html': '<img src="x">' }))).toBe(
            false
        );
    });

    it('treats an empty clipboard as no text', () => {
        expect(ClipboardContent.isOfficeText(null)).toBe(false);
    });
});

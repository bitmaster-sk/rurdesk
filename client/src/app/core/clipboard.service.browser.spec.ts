import { TestBed } from '@angular/core/testing';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ClipboardService } from './clipboard.service';

describe('ClipboardService', () => {
    let service: ClipboardService;

    // navigator.clipboard is a prototype getter; an own property shadows it and
    // `delete` puts the real one back.
    const stubClipboard = (value: unknown): void => {
        Object.defineProperty(navigator, 'clipboard', { value, configurable: true });
    };

    beforeEach(() => {
        TestBed.configureTestingModule({ providers: [ClipboardService] });
        service = TestBed.inject(ClipboardService);
    });

    afterEach(() => {
        delete (navigator as unknown as Record<string, unknown>)['clipboard'];
        vi.restoreAllMocks();
    });

    it('writes through the async clipboard API when it is available', async () => {
        const writeText = vi.fn().mockResolvedValue(undefined);
        stubClipboard({ writeText });

        await expect(service.copy('token-abc')).resolves.toBe(true);
        expect(writeText).toHaveBeenCalledWith('token-abc');
    });

    it('falls back to execCommand when the clipboard API is missing (insecure context)', async () => {
        stubClipboard(undefined);
        const execCommand = vi.spyOn(document, 'execCommand').mockReturnValue(true);

        await expect(service.copy('token-abc')).resolves.toBe(true);
        expect(execCommand).toHaveBeenCalledWith('copy');
    });

    it('falls back to execCommand when the clipboard write rejects', async () => {
        const writeText = vi.fn().mockRejectedValue(new Error('not allowed'));
        stubClipboard({ writeText });
        const execCommand = vi.spyOn(document, 'execCommand').mockReturnValue(true);

        await expect(service.copy('token-abc')).resolves.toBe(true);
        expect(writeText).toHaveBeenCalled();
        expect(execCommand).toHaveBeenCalledWith('copy');
    });

    it('reports failure when both paths fail', async () => {
        stubClipboard(undefined);
        vi.spyOn(document, 'execCommand').mockReturnValue(false);

        await expect(service.copy('token-abc')).resolves.toBe(false);
    });

    it('reports failure when execCommand throws', async () => {
        stubClipboard(undefined);
        vi.spyOn(document, 'execCommand').mockImplementation(() => {
            throw new Error('blocked');
        });

        await expect(service.copy('token-abc')).resolves.toBe(false);
    });

    it('leaves no scratch textarea behind', async () => {
        stubClipboard(undefined);
        vi.spyOn(document, 'execCommand').mockReturnValue(true);
        const before = document.querySelectorAll('textarea').length;

        await service.copy('token-abc');

        expect(document.querySelectorAll('textarea').length).toBe(before);
    });

    it('copies the text the caller passed, not a truncated form', async () => {
        stubClipboard(undefined);
        const longToken = 'rd_'.concat('a1b2c3d4'.repeat(12));
        let selectedValue = '';
        vi.spyOn(document, 'execCommand').mockImplementation(() => {
            selectedValue = (document.activeElement as HTMLTextAreaElement | null)?.value ?? '';
            return true;
        });

        await expect(service.copy(longToken)).resolves.toBe(true);
        expect(selectedValue).toBe(longToken);
    });
});

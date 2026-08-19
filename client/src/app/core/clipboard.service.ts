import { DOCUMENT } from '@angular/common';
import { Injectable, inject } from '@angular/core';

/**
 * Copies text to the system clipboard.
 *
 * `navigator.clipboard` exists only in a secure context (HTTPS or localhost), so
 * on a plain-HTTP deployment the whole API is `undefined` — reading `.writeText`
 * off it throws a TypeError before any promise is created. This service probes
 * for the API and falls back to the deprecated `document.execCommand('copy')`,
 * which still works in insecure contexts.
 */
@Injectable({ providedIn: 'root' })
export class ClipboardService {
    private readonly document = inject(DOCUMENT);

    public async copy(text: string): Promise<boolean> {
        if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
            try {
                await navigator.clipboard.writeText(text);
                return true;
            } catch {
                return this.copyByExecCommand(text);
            }
        }
        return this.copyByExecCommand(text);
    }

    private copyByExecCommand(text: string): boolean {
        const doc = this.document;
        const body = doc.body;
        if (!body) {
            return false;
        }

        const textarea = doc.createElement('textarea');
        textarea.value = text;
        textarea.setAttribute('readonly', '');
        textarea.setAttribute('aria-hidden', 'true');
        textarea.style.position = 'fixed';
        textarea.style.top = '0';
        textarea.style.left = '0';
        textarea.style.opacity = '0';
        textarea.style.pointerEvents = 'none';
        body.appendChild(textarea);

        const selection = doc.getSelection();
        const previousRange =
            selection && selection.rangeCount > 0 ? selection.getRangeAt(0) : null;

        let isCopied: boolean;
        try {
            textarea.select();
            textarea.setSelectionRange(0, text.length);
            isCopied = doc.execCommand('copy');
        } catch {
            isCopied = false;
        } finally {
            textarea.remove();
            if (selection && previousRange) {
                selection.removeAllRanges();
                selection.addRange(previousRange);
            }
        }

        return isCopied;
    }
}

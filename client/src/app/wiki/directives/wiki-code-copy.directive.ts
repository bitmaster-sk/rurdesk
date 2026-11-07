import { DestroyRef, Directive, ElementRef, inject } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MarkdownComponent } from 'ngx-markdown';
import { ClipboardService } from 'src/app/core/clipboard.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { I18nService } from 'src/app/shared/i18n/i18n.service';

/** Marks the wrapper this directive puts around a decorated code block. */
const WRAPPER_ATTRIBUTE = 'data-wiki-code';

/**
 * Adds a copy button above every plain code block of a rendered wiki article.
 *
 * The markdown component replaces the article's inner HTML on every render and
 * only then emits `ready`, so each pass decorates a fresh DOM: the previous
 * buttons are discarded together with the listeners attached to them. A `ready`
 * that arrives without a re-render is covered by the wrapper check, which keeps
 * the pass idempotent.
 */
@Directive({
    selector: '[appWikiCodeCopy]',
    standalone: false
})
export class WikiCodeCopyDirective {
    private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
    private readonly clipboard = inject(ClipboardService);
    private readonly toast = inject(ToastNotificationService);
    private readonly i18n = inject(I18nService);

    /**
     * One controller for the whole lifetime of the directive: a re-render
     * drops the decorated DOM (and with it the buttons and their listeners),
     * so only the final teardown needs an explicit abort.
     */
    private readonly clicks = new AbortController();

    public constructor() {
        const destroyRef = inject(DestroyRef);
        inject(MarkdownComponent, { self: true })
            .ready.pipe(takeUntilDestroyed(destroyRef))
            .subscribe(() => this.decorate());
        destroyRef.onDestroy(() => this.clicks.abort());
    }

    private decorate(): void {
        const label = this.i18n.instant('WIKI.PAGE.COPY_CODE');
        for (const code of Array.from(
            this.host.nativeElement.querySelectorAll<HTMLElement>('pre > code')
        )) {
            // Diagram fences belong to the mermaid and plantuml directives; the
            // language class identifies them whether they already ran or not.
            if (
                code.classList.contains('language-mermaid') ||
                code.classList.contains('language-plantuml')
            ) {
                continue;
            }
            const pre = code.parentElement;
            if (!pre || pre.parentElement?.hasAttribute(WRAPPER_ATTRIBUTE)) {
                continue;
            }
            const button = document.createElement('button');
            button.type = 'button';
            button.className = 'wiki-code__copy';
            // The visible text doubles as the accessible name. The button sits
            // in the wrapper, outside the code element it copies from.
            button.textContent = label;
            button.dataset['testid'] = 'wiki-code-copy';
            button.addEventListener('click', () => void this.copyCode(code), {
                signal: this.clicks.signal
            });
            const wrapper = document.createElement('div');
            wrapper.className = 'wiki-code';
            wrapper.setAttribute(WRAPPER_ATTRIBUTE, '');
            pre.replaceWith(wrapper);
            wrapper.append(button, pre);
        }
    }

    private copyCode(code: HTMLElement): void {
        // textContent carries the block exactly as written, whitespace and
        // HTML escapes decoded included; the button's own label can never leak
        // into the copy because it lives outside the code element.
        void this.clipboard.copy(code.textContent ?? '').then(isCopied => {
            if (isCopied) {
                this.toast.showSuccess('WIKI.PAGE.CODE_COPIED');
            } else {
                this.toast.showError('WIKI.PAGE.CODE_COPY_FAILED');
            }
        });
    }
}

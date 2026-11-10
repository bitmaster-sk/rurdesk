import {
    ApplicationRef,
    ComponentRef,
    DestroyRef,
    Directive,
    ElementRef,
    EnvironmentInjector,
    createComponent,
    inject,
    input
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { TablerIconComponent } from '@tabler/icons-angular';
import { MarkdownComponent } from 'ngx-markdown';
import { ClipboardService } from 'src/app/core/clipboard.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { I18nService } from 'src/app/shared/i18n/i18n.service';

const CODE_BLOCK_ATTRIBUTE = 'data-markdown-code';
const COPY_ICON_ATTRIBUTE = 'data-markdown-code-copy';

@Directive({
    selector: '[appMarkdownCodeCopy]',
    standalone: false
})
export class MarkdownCodeCopyDirective {
    public readonly skipDiagramSource = input(false);

    private readonly destroyRef = inject(DestroyRef);
    private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
    private readonly appRef = inject(ApplicationRef);
    private readonly envInjector = inject(EnvironmentInjector);
    private readonly clipboard = inject(ClipboardService);
    private readonly toast = inject(ToastNotificationService);
    private readonly i18n = inject(I18nService);
    private readonly iconRefs = new Map<HTMLElement, ComponentRef<TablerIconComponent>>();
    private readonly copyTimers = new Map<HTMLElement, ReturnType<typeof setTimeout>>();
    private readonly labelKeys = new Map<HTMLElement, string>();
    private readonly clicks = new AbortController();

    public constructor() {
        inject(MarkdownComponent, { self: true })
            .ready.pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(() => this.decorate());
        this.i18n.langChange$
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(() => this.refreshLabels());
        this.host.nativeElement.addEventListener('click', this.onHostClick, {
            signal: this.clicks.signal
        });
        this.host.nativeElement.addEventListener('keydown', this.onHostKeydown, {
            signal: this.clicks.signal
        });
        this.destroyRef.onDestroy(() => {
            this.clicks.abort();
            for (const button of this.iconRefs.keys()) {
                this.destroyIcon(button);
            }
        });
    }

    private decorate(): void {
        this.destroyDetachedIcons();

        for (const code of Array.from(
            this.host.nativeElement.querySelectorAll<HTMLElement>('pre > code')
        )) {
            if (
                this.skipDiagramSource() &&
                (code.classList.contains('language-mermaid') ||
                    code.classList.contains('language-plantuml'))
            ) {
                continue;
            }

            const pre = code.parentElement;
            if (!pre || pre.parentElement?.hasAttribute(CODE_BLOCK_ATTRIBUTE)) {
                continue;
            }

            const wrapper = document.createElement('div');
            wrapper.className = 'markdown-code-block';
            wrapper.setAttribute(CODE_BLOCK_ATTRIBUTE, '');

            const iconHost = document.createElement('tabler-icon');
            iconHost.className = 'markdown-code-copy';
            iconHost.setAttribute(COPY_ICON_ATTRIBUTE, '');
            iconHost.dataset['testid'] = 'markdown-code-copy';
            iconHost.setAttribute('role', 'button');
            iconHost.setAttribute('tabindex', '0');

            const iconRef = createComponent(TablerIconComponent, {
                environmentInjector: this.envInjector,
                hostElement: iconHost
            });
            iconRef.setInput('icon', 'copy');
            iconRef.setInput('size', 16);
            this.appRef.attachView(iconRef.hostView);
            this.iconRefs.set(iconHost, iconRef);
            this.setLabel(iconHost, 'MARKDOWN.COPY_CODE');

            pre.replaceWith(wrapper);
            wrapper.append(iconHost, pre);
            iconRef.changeDetectorRef.detectChanges();
        }
    }

    private readonly onHostClick = (event: Event): void => {
        if (!(event.target instanceof Element)) {
            return;
        }
        const copyIcon = event.target.closest<HTMLElement>(`[${COPY_ICON_ATTRIBUTE}]`);
        const code = copyIcon?.parentElement?.querySelector<HTMLElement>('pre > code');
        if (copyIcon && code && this.host.nativeElement.contains(copyIcon)) {
            event.preventDefault();
            event.stopPropagation();
            void this.copyCode(copyIcon, code);
        }
    };

    private readonly onHostKeydown = (event: KeyboardEvent): void => {
        if ((event.key !== 'Enter' && event.key !== ' ') || !(event.target instanceof Element)) {
            return;
        }
        const copyIcon = event.target.closest<HTMLElement>(`[${COPY_ICON_ATTRIBUTE}]`);
        const code = copyIcon?.parentElement?.querySelector<HTMLElement>('pre > code');
        if (copyIcon && code && this.host.nativeElement.contains(copyIcon)) {
            event.preventDefault();
            event.stopPropagation();
            void this.copyCode(copyIcon, code);
        }
    };

    private async copyCode(copyIcon: HTMLElement, code: HTMLElement): Promise<void> {
        let isCopied: boolean;
        try {
            isCopied = await this.clipboard.copy(code.textContent ?? '');
        } catch {
            if (!this.destroyRef.destroyed) {
                this.toast.showError('MARKDOWN.CODE_COPY_FAILED');
            }
            return;
        }

        if (this.destroyRef.destroyed) {
            return;
        }
        if (!isCopied) {
            this.toast.showError('MARKDOWN.CODE_COPY_FAILED');
            return;
        }

        this.toast.showSuccess('MARKDOWN.CODE_COPIED');
        this.setLabel(copyIcon, 'MARKDOWN.CODE_COPIED');
        const iconRef = this.iconRefs.get(copyIcon);
        iconRef?.setInput('icon', 'copy-check');
        iconRef?.changeDetectorRef.detectChanges();
        this.clearCopyTimer(copyIcon);
        this.copyTimers.set(
            copyIcon,
            setTimeout(() => {
                if (!this.iconRefs.has(copyIcon)) {
                    return;
                }
                this.setLabel(copyIcon, 'MARKDOWN.COPY_CODE');
                const currentIconRef = this.iconRefs.get(copyIcon);
                currentIconRef?.setInput('icon', 'copy');
                currentIconRef?.changeDetectorRef.detectChanges();
                this.copyTimers.delete(copyIcon);
            }, 1500)
        );
    }

    private destroyDetachedIcons(): void {
        for (const copyIcon of this.iconRefs.keys()) {
            if (!copyIcon.isConnected) {
                this.destroyIcon(copyIcon);
            }
        }
    }

    private destroyIcon(copyIcon: HTMLElement): void {
        this.clearCopyTimer(copyIcon);
        this.labelKeys.delete(copyIcon);
        const iconRef = this.iconRefs.get(copyIcon);
        if (!iconRef) {
            return;
        }
        this.appRef.detachView(iconRef.hostView);
        iconRef.destroy();
        this.iconRefs.delete(copyIcon);
    }

    private refreshLabels(): void {
        for (const [copyIcon, key] of this.labelKeys) {
            this.setLabel(copyIcon, key);
        }
    }

    private setLabel(copyIcon: HTMLElement, key: string): void {
        this.labelKeys.set(copyIcon, key);
        this.i18n
            .get$(key)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(label => {
                if (this.iconRefs.has(copyIcon) && this.labelKeys.get(copyIcon) === key) {
                    copyIcon.setAttribute('aria-label', label);
                    copyIcon.title = label;
                }
            });
    }

    private clearCopyTimer(copyIcon: HTMLElement): void {
        const timer = this.copyTimers.get(copyIcon);
        if (timer !== undefined) {
            clearTimeout(timer);
            this.copyTimers.delete(copyIcon);
        }
    }
}

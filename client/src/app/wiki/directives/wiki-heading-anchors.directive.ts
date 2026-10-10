import { DestroyRef, Directive, ElementRef, inject } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ActivatedRoute, Router } from '@angular/router';
import { MarkdownComponent } from 'ngx-markdown';
import { ClipboardService } from 'src/app/core/clipboard.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { WikiAnchorConverter } from '../converter/wiki-anchor.converter';
import { WikiMermaidDirective } from './wiki-mermaid.directive';

@Directive({
    selector: '[appWikiHeadingAnchors]',
    standalone: false
})
export class WikiHeadingAnchorsDirective {
    private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
    private readonly route = inject(ActivatedRoute);
    private readonly router = inject(Router);
    private readonly clipboard = inject(ClipboardService);
    private readonly toast = inject(ToastNotificationService);
    private readonly i18n = inject(I18nService);

    public constructor() {
        const destroyRef = inject(DestroyRef);
        inject(MarkdownComponent, { self: true })
            .ready.pipe(takeUntilDestroyed(destroyRef))
            .subscribe(() => {
                this.decorate();
                this.scrollTo(this.route.snapshot.fragment);
            });
        this.route.fragment
            .pipe(takeUntilDestroyed(destroyRef))
            .subscribe(fragment => this.scrollTo(fragment));
        // Diagrams settle after `ready` and shift the page layout, so the
        // anchor scroll has to be repeated once they are rendered.
        inject(WikiMermaidDirective, { self: true, optional: true })
            ?.settled.pipe(takeUntilDestroyed(destroyRef))
            .subscribe(() => this.scrollTo(this.route.snapshot.fragment));
    }

    private decorate(): void {
        const headings = Array.from(
            this.host.nativeElement.querySelectorAll<HTMLElement>('h1, h2, h3, h4, h5, h6')
        );
        const anchors = WikiAnchorConverter.toAnchors(
            headings.map(heading => heading.textContent ?? '')
        );
        const label = this.i18n.instant('WIKI.PAGE.HEADING_LINK');
        headings.forEach((heading, index) => {
            const anchor = anchors[index];
            heading.id = WikiAnchorConverter.toElementId(anchor);
            const link = document.createElement('a');
            link.className = 'wiki-heading-anchor';
            link.href = `#${anchor}`;
            link.textContent = '#';
            link.setAttribute('aria-label', label);
            link.dataset['testid'] = 'wiki-heading-anchor';
            link.addEventListener('click', event => this.onAnchorClick(event, anchor));
            heading.append(link);
        });
    }

    private onAnchorClick(event: MouseEvent, anchor: string): void {
        event.preventDefault();
        event.stopPropagation();
        const tree = this.router.createUrlTree([], { relativeTo: this.route, fragment: anchor });
        void this.router.navigateByUrl(tree, { replaceUrl: true });
        void this.clipboard
            .copy(`${window.location.origin}${this.router.serializeUrl(tree)}`)
            .then(isCopied => {
                if (isCopied) {
                    this.toast.showSuccess('WIKI.PAGE.LINK_COPIED');
                }
            });
    }

    private scrollTo(fragment: string | null): void {
        if (!fragment) {
            return;
        }
        const target = this.host.nativeElement.querySelector(
            `#${CSS.escape(WikiAnchorConverter.toElementId(fragment))}`
        );
        target?.scrollIntoView({ block: 'start' });
    }
}

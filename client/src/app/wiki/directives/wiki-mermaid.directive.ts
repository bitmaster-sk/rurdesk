import {
    ApplicationRef,
    ComponentRef,
    DestroyRef,
    Directive,
    ElementRef,
    EnvironmentInjector,
    createComponent,
    inject
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MarkdownComponent } from 'ngx-markdown';
import { MermaidDiagramComponent } from 'src/app/shared/mermaid/mermaid-diagram.component';

/**
 * Replaces rendered ```mermaid code fences with live diagram components.
 * ngx-markdown re-renders the whole article (and fires `ready`) on every data
 * change — every keystroke in the editor preview — so this decorates from
 * scratch each pass: components from the previous `ready` are destroyed and
 * re-created. The render cache makes that cheap and flicker-free for sources
 * that didn't change.
 */
@Directive({
    selector: '[appWikiMermaid]',
    standalone: false
})
export class WikiMermaidDirective {
    private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
    private readonly appRef = inject(ApplicationRef);
    private readonly envInjector = inject(EnvironmentInjector);
    private readonly diagramRefs: ComponentRef<MermaidDiagramComponent>[] = [];

    public constructor() {
        const destroyRef = inject(DestroyRef);
        inject(MarkdownComponent, { self: true })
            .ready.pipe(takeUntilDestroyed(destroyRef))
            .subscribe(() => this.decorate());
        destroyRef.onDestroy(() => this.destroyDiagrams());
    }

    private decorate(): void {
        this.destroyDiagrams();
        const blocks = Array.from(
            this.host.nativeElement.querySelectorAll<HTMLElement>('pre > code.language-mermaid')
        );
        // No diagrams on this page — never touch the loader (it would pull the
        // whole mermaid bundle for nothing).
        if (blocks.length === 0) {
            return;
        }
        for (const code of blocks) {
            const pre = code.parentElement;
            if (!pre) {
                continue;
            }
            // Host carries the component's selector so the DOM matches what a
            // template render would produce (createComponent with a hostElement
            // never wraps it itself).
            const host = document.createElement('app-mermaid-diagram');
            const ref = createComponent(MermaidDiagramComponent, {
                environmentInjector: this.envInjector,
                hostElement: host
            });
            ref.setInput('source', code.textContent?.trim() ?? '');
            this.appRef.attachView(ref.hostView);
            pre.replaceWith(host);
            this.diagramRefs.push(ref);
        }
    }

    private destroyDiagrams(): void {
        for (const ref of this.diagramRefs) {
            this.appRef.detachView(ref.hostView);
            ref.destroy();
        }
        this.diagramRefs.length = 0;
    }
}

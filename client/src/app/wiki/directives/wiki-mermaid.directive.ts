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
import { Subject } from 'rxjs';
import { MarkdownComponent } from 'ngx-markdown';
import { MermaidDiagramComponent } from 'src/app/shared/mermaid/mermaid-diagram.component';

@Directive({
    selector: '[appWikiMermaid]',
    standalone: false
})
export class WikiMermaidDirective {
    /** Render delay in ms passed to every diagram; the editor preview debounces. */
    public readonly mermaidDebounce = input(0);

    /** Emits when all diagrams created for the latest markdown render have settled. */
    public readonly settled = new Subject<void>();

    private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
    private readonly appRef = inject(ApplicationRef);
    private readonly envInjector = inject(EnvironmentInjector);
    private readonly diagramRefs: ComponentRef<MermaidDiagramComponent>[] = [];
    private generation = 0;

    public constructor() {
        const destroyRef = inject(DestroyRef);
        inject(MarkdownComponent, { self: true })
            .ready.pipe(takeUntilDestroyed(destroyRef))
            .subscribe(() => this.decorate());
        destroyRef.onDestroy(() => {
            this.destroyDiagrams();
            this.settled.complete();
        });
    }

    private decorate(): void {
        this.destroyDiagrams();
        this.generation++;
        const blocks = Array.from(
            this.host.nativeElement.querySelectorAll<HTMLElement>('pre > code.language-mermaid')
        );
        // No diagram on the page: never touch the loader, it would pull the
        // whole mermaid bundle for nothing.
        if (blocks.length === 0) {
            return;
        }
        const generation = this.generation;
        let pending = 0;
        for (const code of blocks) {
            const pre = code.parentElement;
            if (!pre) {
                continue;
            }
            const host = document.createElement('app-mermaid-diagram');
            const ref = createComponent(MermaidDiagramComponent, {
                environmentInjector: this.envInjector,
                hostElement: host
            });
            ref.setInput('source', code.textContent?.trim() ?? '');
            ref.setInput('debounce', this.mermaidDebounce());
            // A render can outlive the markdown pass that created it — only
            // count settled for the generation that still owns the component.
            ref.instance.settled.subscribe(() => {
                if (generation !== this.generation || --pending > 0) {
                    return;
                }
                this.settled.next();
            });
            pending++;
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

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
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { PlantUmlDiagramComponent } from 'src/app/shared/plantuml/plantuml-diagram.component';

@Directive({
    selector: '[appWikiPlantUml]',
    standalone: false
})
export class WikiPlantUmlDirective {
    /** Render delay in ms passed to every diagram; the editor preview debounces. */
    public readonly plantUmlDebounce = input(0);

    /** Emits when all diagrams created for the latest markdown render have settled. */
    public readonly settled = new Subject<void>();

    private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
    private readonly appRef = inject(ApplicationRef);
    private readonly envInjector = inject(EnvironmentInjector);
    private readonly settings = inject(SettingsStore);
    private readonly diagramRefs: ComponentRef<PlantUmlDiagramComponent>[] = [];
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
        // Disabled plantuml leaves the block exactly as markdown rendered it;
        // the diagram component also checks, but here it would create one just
        // to show a code block that is already on the page.
        if (!this.settings.isPlantUmlEnabled()) {
            return;
        }
        const blocks = Array.from(
            this.host.nativeElement.querySelectorAll<HTMLElement>('pre > code.language-plantuml')
        );
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
            const host = document.createElement('app-plantuml-diagram');
            const ref = createComponent(PlantUmlDiagramComponent, {
                environmentInjector: this.envInjector,
                hostElement: host
            });
            ref.setInput('source', code.textContent?.trim() ?? '');
            ref.setInput('debounce', this.plantUmlDebounce());
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

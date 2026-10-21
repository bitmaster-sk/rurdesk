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
import { PlantUmlDiagramComponent } from 'src/app/shared/plantuml/plantuml-diagram.component';

@Directive({
    selector: '[appWikiPlantUml]',
    standalone: false
})
export class WikiPlantUmlDirective {
    public readonly plantUmlDebounce = input(0);

    public readonly settled = new Subject<void>();

    private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
    private readonly appRef = inject(ApplicationRef);
    private readonly envInjector = inject(EnvironmentInjector);
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

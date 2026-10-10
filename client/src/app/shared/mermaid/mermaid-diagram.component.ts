import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    ElementRef,
    Injector,
    ViewEncapsulation,
    afterNextRender,
    effect,
    inject,
    input,
    output,
    signal,
    viewChild
} from '@angular/core';
import { MermaidRenderService, MermaidResult } from './mermaid-render.service';

type DiagramState = { kind: 'loading' } | { kind: 'svg'; svg: string } | { kind: 'error' };

@Component({
    selector: 'app-mermaid-diagram',
    templateUrl: './mermaid-diagram.component.html',
    styleUrls: ['./mermaid-diagram.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    // The svg arrives via innerHTML without the encapsulation attribute, so scoped
    // rules cannot reach it — keep this unscoped if touching this line.
    encapsulation: ViewEncapsulation.None,
    standalone: false
})
export class MermaidDiagramComponent {
    public readonly source = input.required<string>();

    /** Render delay in ms; 0 renders immediately. Only the wiki editor preview debounces. */
    public readonly debounce = input(0);

    /** Emits once the state settles to svg or error (never for loading). */
    public readonly settled = output<void>();

    private readonly renderService = inject(MermaidRenderService);
    private readonly svgHost = viewChild<ElementRef<HTMLDivElement>>('svgHost');
    private readonly injector = inject(Injector);

    protected readonly state = signal<DiagramState>({ kind: 'loading' });

    private timer: ReturnType<typeof setTimeout> | null = null;
    private attempt = 0;

    public constructor() {
        effect(() => this.applySource(this.source()));
        inject(DestroyRef).onDestroy(() => this.clearTimer());
    }

    private applySource(source: string): void {
        this.clearTimer();
        const cached = this.renderService.cached(source);
        if (cached) {
            this.settle(cached);
            return;
        }
        this.state.set({ kind: 'loading' });
        const attempt = ++this.attempt;
        const delay = this.debounce();
        if (delay > 0) {
            this.timer = setTimeout(() => this.render(source, attempt), delay);
        } else {
            this.render(source, attempt);
        }
    }

    private render(source: string, attempt: number): void {
        this.timer = null;
        void this.renderService.render(source).then(result => {
            // A newer source may have arrived while this render was in flight.
            if (attempt !== this.attempt) {
                return;
            }
            this.settle(result);
        });
    }

    private settle(result: MermaidResult): void {
        const state: DiagramState =
            'svg' in result ? { kind: 'svg', svg: result.svg } : { kind: 'error' };
        this.state.set(state);
        afterNextRender(
            {
                write: () => {
                    if (this.state() !== state) {
                        return;
                    }
                    const host = this.svgHost();
                    if (state.kind === 'svg' && host) {
                        // Written to the DOM directly: Angular's [innerHTML] sanitizer
                        // strips svg; trust comes from mermaid's securityLevel 'strict'.
                        host.nativeElement.innerHTML = state.svg;
                        this.useNaturalWidth(host.nativeElement.querySelector('svg'));
                    }
                    this.settled.emit();
                }
            },
            { injector: this.injector }
        );
    }

    private useNaturalWidth(svg: SVGSVGElement | null): void {
        const naturalWidth = svg?.style.maxWidth;
        if (!svg || !naturalWidth) {
            return;
        }
        svg.removeAttribute('width');
        svg.style.width = naturalWidth;
        svg.style.maxWidth = 'none';
    }

    private clearTimer(): void {
        if (this.timer !== null) {
            clearTimeout(this.timer);
            this.timer = null;
        }
    }
}

import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    ElementRef,
    ViewEncapsulation,
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

    protected readonly state = signal<DiagramState>({ kind: 'loading' });

    private timer: ReturnType<typeof setTimeout> | null = null;
    private attempt = 0;

    public constructor() {
        effect(() => this.applySource(this.source()));
        effect(() => {
            const state = this.state();
            const host = this.svgHost();
            if (state.kind === 'svg' && host) {
                // Written to the DOM directly: Angular's [innerHTML] sanitizer
                // strips svg; trust comes from mermaid's securityLevel 'strict'.
                host.nativeElement.innerHTML = state.svg;
            }
        });
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
        this.state.set('svg' in result ? { kind: 'svg', svg: result.svg } : { kind: 'error' });
        this.settled.emit();
    }

    private clearTimer(): void {
        if (this.timer !== null) {
            clearTimeout(this.timer);
            this.timer = null;
        }
    }
}

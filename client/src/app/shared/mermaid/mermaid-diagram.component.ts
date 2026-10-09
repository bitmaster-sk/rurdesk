import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    ElementRef,
    ViewEncapsulation,
    effect,
    inject,
    input,
    signal,
    viewChild
} from '@angular/core';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { MermaidRenderService, MermaidResult } from './mermaid-render.service';

type DiagramState = { kind: 'loading' } | { kind: 'svg'; svg: string } | { kind: 'error' };

/** Debounce so the editor preview (re-renders on every keystroke) stays cheap. */
const DEBOUNCE_MS = 300;

/**
 * Renders one fenced mermaid block. A cached result is applied synchronously —
 * no debounce, no flicker — because the wiki directive destroys and recreates
 * this component on every editor keystroke and the cache makes repeats free.
 * A cache miss shows the source in a `<pre>` with a loading label while the
 * debounced render runs.
 *
 * The SVG is written straight into the host element rather than through
 * Angular's `[innerHTML]`, which sanitizes away SVG; trust comes from mermaid's
 * `securityLevel: 'strict'` (see MermaidLoaderService).
 */
@Component({
    selector: 'app-mermaid-diagram',
    templateUrl: './mermaid-diagram.component.html',
    styleUrls: ['./mermaid-diagram.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    // Unscoped on purpose: the svg arrives via innerHTML and never carries the
    // encapsulation attribute, so scoped rules can't reach it (same reason as
    // DiffViewer). Class names carry the `mermaid-diagram__` prefix instead.
    encapsulation: ViewEncapsulation.None,
    standalone: false
})
export class MermaidDiagramComponent {
    public readonly source = input.required<string>();

    private readonly renderService = inject(MermaidRenderService);
    private readonly i18n = inject(I18nService);
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
                host.nativeElement.innerHTML = state.svg;
            }
        });
        inject(DestroyRef).onDestroy(() => this.clearTimer());
    }

    protected get loadingLabel(): string {
        return this.i18n.instant('SHARED.MERMAID.LOADING');
    }

    protected get errorLabel(): string {
        return this.i18n.instant('SHARED.MERMAID.ERROR');
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
        this.timer = setTimeout(() => {
            this.timer = null;
            void this.renderService.render(source).then(result => {
                // A newer source may have arrived while we debounced.
                if (attempt !== this.attempt) {
                    return;
                }
                this.settle(result);
            });
        }, DEBOUNCE_MS);
    }

    private settle(result: MermaidResult): void {
        this.state.set('svg' in result ? { kind: 'svg', svg: result.svg } : { kind: 'error' });
    }

    private clearTimer(): void {
        if (this.timer !== null) {
            clearTimeout(this.timer);
            this.timer = null;
        }
    }
}

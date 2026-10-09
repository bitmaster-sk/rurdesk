import { Injectable, inject } from '@angular/core';
import { MermaidLoaderService } from './mermaid-loader.service';

/** A rendered diagram: either the SVG markup or the failure message. */
export type MermaidResult = { svg: string } | { error: string };

/**
 * Renders mermaid sources and memoizes the outcome in a Map keyed by source.
 * `cached()` answers synchronously so re-renders of the same source (the wiki
 * editor preview fires on every keystroke) swap the SVG in without another
 * debounce round-trip — no blink, no render cost.
 *
 * Renders are serialized on one internal promise chain: mermaid keeps
 * module-level state between renders, so two diagrams in flight at once cannot
 * be trusted to stay independent.
 */
@Injectable({ providedIn: 'root' })
export class MermaidRenderService {
    private readonly loader = inject(MermaidLoaderService);
    private readonly cache = new Map<string, MermaidResult>();
    private queue: Promise<unknown> = Promise.resolve();
    private nextId = 1;

    /** Synchronous lookup of a previously rendered source, else `null`. */
    public cached(source: string): MermaidResult | null {
        return this.cache.get(source) ?? null;
    }

    /** Returns the cached result or renders the source. Never rejects. */
    public render(source: string): Promise<MermaidResult> {
        const cached = this.cached(source);
        if (cached) {
            return Promise.resolve(cached);
        }
        const run = this.queue.then(() => this.renderNow(source));
        // The outcome flows to `run`'s consumer; the queue only tracks
        // completion, so a failed render keeps the chain alive.
        this.queue = run.then(
            () => undefined,
            () => undefined
        );
        return run;
    }

    private async renderNow(source: string): Promise<MermaidResult> {
        const mermaid = await this.loader.load();
        const id = `mermaid-${this.nextId++}`;
        try {
            const { svg } = await mermaid.render(id, source);
            const result: MermaidResult = { svg };
            this.cache.set(source, result);
            return result;
        } catch (error) {
            // `suppressErrorRendering` makes mermaid throw instead of drawing
            // an error bomb — but it leaves its scratch `#d<id>` element in
            // the body, which would leak one div per failed render.
            document.getElementById(`d${id}`)?.remove();
            const result: MermaidResult = {
                error: error instanceof Error ? error.message : String(error)
            };
            this.cache.set(source, result);
            return result;
        }
    }
}

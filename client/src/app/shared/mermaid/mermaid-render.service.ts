import { Injectable, inject } from '@angular/core';
import type { Mermaid } from 'mermaid';
import { MermaidLoaderService } from './mermaid-loader.service';

/** A rendered diagram: either the SVG markup or the failure message. */
export type MermaidResult = { svg: string } | { error: string };

/** Maximum cached results; the oldest entry is evicted beyond that. */
const CACHE_LIMIT = 50;

/**
 * Renders mermaid sources and memoizes the outcome in a Map keyed by source
 * (`cached()` answers synchronously so re-renders of the same source — the
 * wiki editor preview re-renders on every keystroke — swap the SVG in without
 * a debounce round-trip). Renders are serialized on one internal promise
 * chain: mermaid keeps module-level state between renders.
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
        this.queue = run.then(
            () => undefined,
            () => undefined
        );
        return run;
    }

    private async renderNow(source: string): Promise<MermaidResult> {
        let mermaid: Mermaid;
        try {
            mermaid = await this.loader.load();
        } catch (error) {
            // Load failures are not cached: the loader itself retries the
            // import on the next call, so a later render can still succeed.
            return { error: error instanceof Error ? error.message : String(error) };
        }
        const id = `mermaid-${this.nextId++}`;
        try {
            const { svg } = await mermaid.render(id, source);
            const result: MermaidResult = { svg };
            this.cacheResult(source, result);
            return result;
        } catch (error) {
            // `suppressErrorRendering` makes mermaid throw instead of drawing
            // an error bomb — but it still leaves a scratch `#d<id>` element
            // in the body, which would leak one node per failed render.
            document.getElementById(`d${id}`)?.remove();
            const result: MermaidResult = {
                error: error instanceof Error ? error.message : String(error)
            };
            this.cacheResult(source, result);
            return result;
        }
    }

    private cacheResult(source: string, result: MermaidResult): void {
        this.cache.set(source, result);
        while (this.cache.size > CACHE_LIMIT) {
            const oldest = this.cache.keys().next();
            if (oldest.done) {
                return;
            }
            this.cache.delete(oldest.value);
        }
    }
}

import { HttpErrorResponse } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { PlantUmlApi } from './plantuml.api.service';

/** A render outcome: either the SVG or the failure mode. */
export type PlantUmlResult =
    { svg: string } | { error: 'syntax' | 'unavailable'; message?: string };

/** Maximum cached results; the oldest entry is evicted beyond that. */
const CACHE_LIMIT = 50;

/**
 * Renders plantuml sources through the tracker API and memoizes the outcome in
 * a Map keyed by source (`cached()` answers synchronously so re-renders of the
 * same source — the wiki editor preview re-renders on every keystroke — swap
 * the SVG in without a round-trip). The API is the only network hop: the
 * client never talks to the PlantUML server directly. Never rejects.
 */
@Injectable({ providedIn: 'root' })
export class PlantUmlRenderService {
    private readonly api = inject(PlantUmlApi);
    private readonly cache = new Map<string, PlantUmlResult>();

    /** Synchronous lookup of a previously rendered source, else `null`. */
    public cached(source: string): PlantUmlResult | null {
        return this.cache.get(source) ?? null;
    }

    /** Returns the cached result or renders the source. Never rejects. */
    public render(source: string): Promise<PlantUmlResult> {
        const cached = this.cached(source);
        if (cached) {
            return Promise.resolve(cached);
        }
        return new Promise<PlantUmlResult>(resolve => {
            this.api.render$(source).subscribe({
                next: res => {
                    const result: PlantUmlResult = { svg: res.svg };
                    this.cacheResult(source, result);
                    resolve(result);
                },
                error: err => resolve(this.toError(err))
            });
        });
    }

    // Failure results are not cached: a syntax fix or the server coming back
    // must be able to produce a different outcome on the next render.
    private toError(err: unknown): PlantUmlResult {
        if (err instanceof HttpErrorResponse && err.status === 422) {
            const message = (err.error as { error?: string } | null)?.error;
            return message ? { error: 'syntax', message } : { error: 'syntax' };
        }
        return { error: 'unavailable' };
    }

    private cacheResult(source: string, result: PlantUmlResult): void {
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

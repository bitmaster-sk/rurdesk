import { Injectable, inject } from '@angular/core';
import { ApiError } from '../model/api-error.model';
import { PlantUmlApi } from './plantuml.api.service';

export type PlantUmlResult =
    { svg: string } | { error: 'syntax' | 'unavailable'; message?: string };

const CACHE_LIMIT = 50;

@Injectable({ providedIn: 'root' })
export class PlantUmlRenderService {
    private readonly api = inject(PlantUmlApi);
    private readonly cache = new Map<string, PlantUmlResult>();

    public cached(source: string): PlantUmlResult | null {
        return this.cache.get(source) ?? null;
    }

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
    // ErrorInterceptor rethrows a spread of HttpErrorResponse (not an instanceof),
    // so the status and body are read off the raw object instead.
    private toError(err: unknown): PlantUmlResult {
        const status = (err as { status?: unknown } | null | undefined)?.status;
        if (typeof status === 'number' && status === 422) {
            const message = ApiError.bodyOf(err)?.message;
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

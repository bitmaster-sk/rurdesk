import { ErrorHandler } from '@angular/core';

// The error message Chrome/Firefox produce when a dynamic import() fails
// because the target module could not be loaded (404, network error, wrong
// MIME type). Matching this is more robust than instanceof TypeError because
// the browser wraps the failure in a generic TypeError.
const DYNAMIC_IMPORT_FAILURE = 'Failed to fetch dynamically imported module';

// sessionStorage key used to guard against infinite reload loops when the
// server is genuinely broken (always returning HTML for .js, etc.).
const RELOAD_GUARD_KEY = 'dynamic-import-reloaded';

/**
 * Detects failed dynamic imports (stale chunk references after a redeploy or
 * tab duplicate) and reloads the page so the browser re-fetches index.html
 * with the current chunk hashes. A sessionStorage flag limits recovery to a
 * single attempt per tab to avoid an infinite loop in a broken deployment.
 */
export class DynamicImportErrorHandler implements ErrorHandler {
    public handleError(error: unknown): void {
        const message = error instanceof Error ? error.message : String(error);

        if (message.includes(DYNAMIC_IMPORT_FAILURE) && !this.alreadyReloaded()) {
            this.markReloaded();
            window.location.reload();
            return;
        }

        // Fall back to Angular's default behaviour for all other errors.
        console.error(error);
    }

    private alreadyReloaded(): boolean {
        return sessionStorage.getItem(RELOAD_GUARD_KEY) === '1';
    }

    private markReloaded(): void {
        sessionStorage.setItem(RELOAD_GUARD_KEY, '1');
    }
}

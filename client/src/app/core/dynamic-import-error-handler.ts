import { ErrorHandler } from '@angular/core';

// The error message Chrome/Firefox produce when a dynamic import() fails
// because the target module could not be loaded (404, network error, wrong
// MIME type). Matching this is more robust than instanceof TypeError because
// the browser wraps the failure in a generic TypeError.
const DYNAMIC_IMPORT_FAILURE = 'Failed to fetch dynamically imported module';

// sessionStorage key holding the timestamp of the last recovery reload.
const RELOAD_GUARD_KEY = 'dynamic-import-reloaded-at';

// The guard is a cooldown, not a once-per-tab flag, and must stay one: the app
// shell boots fine even when a lazy chunk 404s, so a flag cleared on bootstrap
// would loop forever, while a flag never cleared would leave the tab unable to
// recover from a second stale chunk — and Chrome copies sessionStorage into a
// duplicated tab, which is exactly the case this handler exists for.
const RELOAD_COOLDOWN_MS = 10_000;

/**
 * Detects failed dynamic imports (stale chunk references after a redeploy or
 * tab duplicate) and reloads the page so the browser re-fetches index.html
 * with the current chunk hashes.
 */
export class DynamicImportErrorHandler implements ErrorHandler {
    public handleError(error: unknown): void {
        const message = error instanceof Error ? error.message : String(error);

        if (message.includes(DYNAMIC_IMPORT_FAILURE) && this.canReload()) {
            this.markReloaded();
            window.location.reload();
            return;
        }

        // Fall back to Angular's default behaviour for all other errors.
        console.error(error);
    }

    private canReload(): boolean {
        const reloadedAt = Number(sessionStorage.getItem(RELOAD_GUARD_KEY));

        if (!Number.isFinite(reloadedAt) || reloadedAt <= 0) {
            return true;
        }

        return Date.now() - reloadedAt > RELOAD_COOLDOWN_MS;
    }

    private markReloaded(): void {
        sessionStorage.setItem(RELOAD_GUARD_KEY, String(Date.now()));
    }
}

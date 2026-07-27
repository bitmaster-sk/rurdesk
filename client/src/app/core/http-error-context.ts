import { HttpContext, HttpContextToken } from '@angular/common/http';

/**
 * Suppresses the global error toast for a single request.
 *
 * ErrorInterceptor toasts every failed response, because the API returns a
 * uniform `{ code, message, translateKey }` body and most callers would
 * otherwise fail silently. Set this where that is wrong:
 *
 * - the failure is expected (a GET that 404s when the resource has never been
 *   created), or
 * - the caller shows its own, more specific message.
 */
export const SILENCE_ERROR_TOAST = new HttpContextToken<boolean>(() => false);

/** Shorthand for `{ context: silentErrors() }` on an HttpClient call. */
export function silentErrors(): HttpContext {
    return new HttpContext().set(SILENCE_ERROR_TOAST, true);
}

/**
 * Body the API renders for every failed request (see the backend `ErrorRenderer`
 * middleware + `internal/errs`). `translateKey` is `omitempty` on the Go side.
 */
export interface ApiErrorBody {
    code: string;
    message: string;
    translateKey?: string;
}

export abstract class ApiError {
    public static translateKeyOf(error: unknown): string | null {
        if (!ApiError.isRecord(error)) {
            return null;
        }
        const body = error['error'];
        if (!ApiError.isRecord(body)) {
            return null;
        }
        const translateKey = body['translateKey'];
        return typeof translateKey === 'string' ? translateKey : null;
    }

    /**
     * Reads the body off its carrier — both `HttpErrorResponse` and the plain object
     * `ErrorInterceptor` re-throws (a spread of one, so `instanceof` no longer holds there).
     * All three fields are validated so a future consumer that needs the full body
     * gets a trustworthy `Partial<ApiErrorBody>`. `translateKeyOf` does NOT use this —
     * it reads `translateKey` directly so a bad `code`/`message` can never block the key.
     */
    public static bodyOf(error: unknown): Partial<ApiErrorBody> | null {
        if (!ApiError.isRecord(error)) {
            return null;
        }
        const body = error['error'];
        return ApiError.isBody(body) ? body : null;
    }

    private static isRecord(value: unknown): value is Record<string, unknown> {
        return value !== null && typeof value === 'object';
    }

    private static isBody(value: unknown): value is Partial<ApiErrorBody> {
        return (
            ApiError.isRecord(value) &&
            (value['code'] === undefined || typeof value['code'] === 'string') &&
            (value['message'] === undefined || typeof value['message'] === 'string') &&
            (value['translateKey'] === undefined || typeof value['translateKey'] === 'string')
        );
    }
}

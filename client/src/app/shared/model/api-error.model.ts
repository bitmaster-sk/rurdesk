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
        return ApiError.bodyOf(error)?.translateKey ?? null;
    }

    /**
     * Reads the body off its carrier — both `HttpErrorResponse` and the plain object
     * `ErrorInterceptor` re-throws (a spread of one, so `instanceof` no longer holds there).
     * Fields are validated but not required: a body is still usable when the API omits one.
     */
    public static bodyOf(error: unknown): Partial<ApiErrorBody> | null {
        if (!ApiError.isRecord(error)) {
            return null;
        }
        const body = error['error'];
        return ApiError.isBody(body) ? body : null;
    }

    private static isBody(value: unknown): value is Partial<ApiErrorBody> {
        return (
            ApiError.isRecord(value) &&
            ApiError.isOptionalString(value['code']) &&
            ApiError.isOptionalString(value['message']) &&
            ApiError.isOptionalString(value['translateKey'])
        );
    }

    private static isRecord(value: unknown): value is Record<string, unknown> {
        return value !== null && typeof value === 'object';
    }

    private static isOptionalString(value: unknown): boolean {
        return value === undefined || typeof value === 'string';
    }
}

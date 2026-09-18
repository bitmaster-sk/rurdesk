import { DynamicImportErrorHandler } from './dynamic-import-error-handler';

const GUARD_KEY = 'dynamic-import-reloaded-at';

const sessionStorageMock = (() => {
    let store: Record<string, string> = {};
    return {
        clear: () => {
            store = {};
        },
        getItem: (key: string) => store[key] ?? null,
        setItem: (key: string, value: string) => {
            store[key] = value;
        }
    };
})();

function makeHandler(): DynamicImportErrorHandler {
    sessionStorageMock.clear();
    return new DynamicImportErrorHandler();
}

function importFailure(): TypeError {
    return new TypeError('Failed to fetch dynamically imported module: http://host/chunk-ABC.js');
}

describe('DynamicImportErrorHandler', () => {
    let reload: ReturnType<typeof vi.fn>;

    beforeEach(() => {
        reload = vi.fn();
        vi.stubGlobal('sessionStorage', sessionStorageMock);
        vi.stubGlobal('window', { location: { reload } });
        vi.spyOn(console, 'error').mockImplementation(() => undefined);
    });

    afterEach(() => {
        sessionStorageMock.clear();
        vi.unstubAllGlobals();
        vi.restoreAllMocks();
    });

    it('reloads the page when a dynamic import failure occurs', () => {
        const handler = makeHandler();

        handler.handleError(importFailure());

        expect(reload).toHaveBeenCalledOnce();
        expect(Number(sessionStorageMock.getItem(GUARD_KEY))).toBeGreaterThan(0);
    });

    it('does not reload for a non-import error', () => {
        const handler = makeHandler();

        const error = new Error('something else broke');

        handler.handleError(error);

        expect(reload).not.toHaveBeenCalled();
        expect(sessionStorageMock.getItem(GUARD_KEY)).toBeNull();
        expect(console.error).toHaveBeenCalledWith(error);
    });

    it('does not reload twice within the cooldown window', () => {
        const handler = makeHandler();

        handler.handleError(importFailure());
        handler.handleError(importFailure());

        expect(reload).toHaveBeenCalledOnce();
    });

    it('reloads again once the cooldown window has passed', () => {
        const handler = makeHandler();
        sessionStorageMock.setItem(GUARD_KEY, String(Date.now() - 60_000));

        handler.handleError(importFailure());

        expect(reload).toHaveBeenCalledOnce();
    });

    it('reloads in a duplicated tab that inherited a stale guard timestamp', () => {
        const handler = makeHandler();
        sessionStorageMock.setItem(GUARD_KEY, String(Date.now() - 5 * 60_000));

        handler.handleError(importFailure());

        expect(reload).toHaveBeenCalledOnce();
    });

    it('ignores a corrupted guard value and still reloads', () => {
        const handler = makeHandler();
        sessionStorageMock.setItem(GUARD_KEY, 'not-a-timestamp');

        handler.handleError(importFailure());

        expect(reload).toHaveBeenCalledOnce();
    });
});

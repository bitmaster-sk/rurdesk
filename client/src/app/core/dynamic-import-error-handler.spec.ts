import { DynamicImportErrorHandler } from './dynamic-import-error-handler';

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

describe('DynamicImportErrorHandler', () => {
    beforeEach(() => {
        vi.stubGlobal('sessionStorage', sessionStorageMock);
        vi.stubGlobal('window', { location: { reload: vi.fn() } });
    });

    afterEach(() => {
        sessionStorageMock.clear();
        vi.unstubAllGlobals();
    });

    it('reloads the page when a dynamic import failure occurs', () => {
        const handler = makeHandler();
        const reload = vi.fn();
        vi.stubGlobal('window', { location: { reload } });

        handler.handleError(
            new TypeError('Failed to fetch dynamically imported module: http://host/chunk-ABC.js')
        );

        expect(reload).toHaveBeenCalledOnce();
        expect(sessionStorageMock.getItem('dynamic-import-reloaded')).toBe('1');
    });

    it('does not reload for a non-import error', () => {
        const handler = makeHandler();
        const reload = vi.fn();
        vi.stubGlobal('window', { location: { reload } });

        handler.handleError(new Error('something else broke'));

        expect(reload).not.toHaveBeenCalled();
        expect(sessionStorageMock.getItem('dynamic-import-reloaded')).toBeNull();
    });

    it('does not reload a second time (infinite-loop guard)', () => {
        const handler = makeHandler();
        const reload = vi.fn();
        vi.stubGlobal('window', { location: { reload } });
        sessionStorageMock.setItem('dynamic-import-reloaded', '1');

        handler.handleError(
            new TypeError('Failed to fetch dynamically imported module: http://host/chunk-XYZ.js')
        );

        expect(reload).not.toHaveBeenCalled();
    });
});

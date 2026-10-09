import { Injector, runInInjectionContext } from '@angular/core';
import { Observable, Subject, of, throwError } from 'rxjs';
import { AttachmentApi } from '../api/attachment.api.service';
import { AttachmentStore } from './attachment.store';

function buildStore(download$: (id: string) => Observable<Blob>): {
    store: AttachmentStore;
    download: ReturnType<typeof vi.fn>;
} {
    const download = vi.fn(download$);
    const injector = Injector.create({
        providers: [{ provide: AttachmentApi, useValue: { download$: download } }]
    });
    const store = runInInjectionContext(injector, () => new AttachmentStore());
    return { store, download };
}

describe('AttachmentStore', () => {
    beforeEach(() => {
        vi.spyOn(URL, 'createObjectURL').mockImplementation(() => 'blob:fake');
        vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined);
    });

    afterEach(() => {
        vi.restoreAllMocks();
    });

    it('reports loading until the blob arrives, then ready with an object url', () => {
        const pending = new Subject<Blob>();
        const { store } = buildStore(() => pending);

        store.acquire('a');
        expect(store.state('a')).toEqual({ status: 'loading' });

        pending.next(new Blob(['abc'], { type: 'image/png' }));
        expect(store.state('a')).toEqual({
            status: 'ready',
            url: 'blob:fake',
            mimeType: 'image/png',
            size: 3
        });
    });

    it('downloads each attachment only once while it is shown', () => {
        const { store, download } = buildStore(() => of(new Blob(['x'])));

        store.acquire('a');
        store.acquire('a');

        expect(download).toHaveBeenCalledTimes(1);
    });

    it('marks the attachment missing when the download fails', () => {
        const { store } = buildStore(() => throwError(() => new Error('404')));

        store.acquire('gone');

        expect(store.state('gone')).toEqual({ status: 'missing' });
    });

    it('keeps the attachment while another holder still shows it', () => {
        const { store } = buildStore(() => of(new Blob(['x'])));
        store.acquire('a');
        store.acquire('a');

        store.release('a');

        expect(store.state('a')?.status).toBe('ready');
        expect(URL.revokeObjectURL).not.toHaveBeenCalled();
    });

    it('frees the object url once nothing shows the attachment', () => {
        const { store } = buildStore(() => of(new Blob(['x'])));
        store.acquire('a');

        store.release('a');

        expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:fake');
        expect(store.state('a')).toBeNull();
    });

    it('tries a missing attachment again after it was released', () => {
        const { store, download } = buildStore(() => throwError(() => new Error('503')));
        store.acquire('a');
        store.release('a');

        store.acquire('a');

        expect(download).toHaveBeenCalledTimes(2);
    });

    it('cancels a download nobody waits for anymore', () => {
        const pending = new Subject<Blob>();
        const { store } = buildStore(() => pending);
        store.acquire('a');

        store.release('a');
        pending.next(new Blob(['x']));

        expect(URL.createObjectURL).not.toHaveBeenCalled();
        expect(store.state('a')).toBeNull();
    });

    it('revokes every object url when cleared', () => {
        const { store } = buildStore(() => of(new Blob(['x'])));
        store.acquire('a');
        store.acquire('b');

        store.revokeAll();

        expect(URL.revokeObjectURL).toHaveBeenCalledTimes(2);
        expect(store.state('a')).toBeNull();
    });
});

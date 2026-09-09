import { DOCUMENT } from '@angular/common';
import { DestroyRef, Injectable, inject } from '@angular/core';
import { Observable, Subject, fromEvent, merge } from 'rxjs';
import { filter, map } from 'rxjs/operators';

const CHANNEL_NAME = 'rurdesk-tracker';

@Injectable({
    providedIn: 'root'
})
export class TrackerSyncService {
    private readonly document = inject(DOCUMENT);

    private readonly destroyRef = inject(DestroyRef);

    private readonly channel = this.openChannel();

    private readonly _remoteChange$ = new Subject<void>();

    public readonly changed$: Observable<void> = merge(
        this._remoteChange$,
        fromEvent(this.document, 'visibilitychange').pipe(
            filter(() => this.document.visibilityState === 'visible'),
            map(() => undefined)
        )
    );

    public constructor() {
        if (this.channel) {
            this.channel.onmessage = () => this._remoteChange$.next();
            this.destroyRef.onDestroy(() => this.channel?.close());
        }
    }

    public publish(): void {
        this.channel?.postMessage(Date.now());
    }

    private openChannel(): BroadcastChannel | null {
        if (typeof BroadcastChannel === 'undefined') {
            return null;
        }
        return new BroadcastChannel(CHANNEL_NAME);
    }
}

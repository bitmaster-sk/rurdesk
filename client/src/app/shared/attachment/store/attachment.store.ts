import { Injectable, OnDestroy, inject, signal } from '@angular/core';
import { Subscription } from 'rxjs';
import { AttachmentApi } from '../api/attachment.api.service';
import { AttachmentState } from '../entity/attachment-state.entity';

@Injectable({ providedIn: 'root' })
export class AttachmentStore implements OnDestroy {
    private readonly api = inject(AttachmentApi);
    private readonly states = signal<ReadonlyMap<string, AttachmentState>>(new Map());
    private readonly holders = new Map<string, number>();
    private readonly downloads = new Map<string, Subscription>();

    public state(idAttachment: string): AttachmentState | null {
        return this.states().get(idAttachment) ?? null;
    }

    public acquire(idAttachment: string): void {
        this.holders.set(idAttachment, (this.holders.get(idAttachment) ?? 0) + 1);
        if (this.states().has(idAttachment)) {
            return;
        }
        this.set(idAttachment, { status: 'loading' });
        const download = this.api.download$(idAttachment).subscribe({
            next: blob =>
                this.set(idAttachment, {
                    status: 'ready',
                    url: URL.createObjectURL(blob),
                    mimeType: blob.type,
                    size: blob.size
                }),
            error: () => this.set(idAttachment, { status: 'missing' })
        });
        if (!download.closed) {
            this.downloads.set(idAttachment, download);
        }
    }

    public release(idAttachment: string): void {
        const count = this.holders.get(idAttachment);
        if (count === undefined) {
            return;
        }
        if (count > 1) {
            this.holders.set(idAttachment, count - 1);
            return;
        }
        this.holders.delete(idAttachment);
        this.drop(idAttachment);
    }

    public revokeAll(): void {
        for (const idAttachment of Array.from(this.states().keys())) {
            this.drop(idAttachment);
        }
        this.holders.clear();
    }

    public ngOnDestroy(): void {
        this.revokeAll();
    }

    private drop(idAttachment: string): void {
        this.downloads.get(idAttachment)?.unsubscribe();
        this.downloads.delete(idAttachment);
        const state = this.states().get(idAttachment);
        if (state?.status === 'ready') {
            URL.revokeObjectURL(state.url);
        }
        this.states.update(current => {
            const next = new Map(current);
            next.delete(idAttachment);
            return next;
        });
    }

    private set(idAttachment: string, state: AttachmentState): void {
        if (state.status !== 'loading') {
            this.downloads.delete(idAttachment);
        }
        this.states.update(current => new Map(current).set(idAttachment, state));
    }
}

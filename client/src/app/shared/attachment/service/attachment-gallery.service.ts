import { Injectable, computed, signal } from '@angular/core';
import { GalleryImage } from '../entity/gallery-image.entity';

@Injectable()
export class AttachmentGallery {
    private readonly groups = signal<ReadonlyMap<object, GalleryImage[]>>(new Map());
    private readonly activeId = signal<string | null>(null);

    public readonly images = computed(() => Array.from(this.groups().values()).flat());
    public readonly activeIndex = computed(() => {
        const idActive = this.activeId();
        const index = this.images().findIndex(image => image.idAttachment === idActive);
        return index < 0 ? null : index;
    });
    public readonly active = computed(() => {
        const index = this.activeIndex();
        return index === null ? null : this.images()[index];
    });
    public readonly isOpen = computed(() => this.active() !== null);

    public register(owner: object, images: GalleryImage[]): void {
        this.groups.update(current => new Map(current).set(owner, images));
    }

    public unregister(owner: object): void {
        this.groups.update(current => {
            const next = new Map(current);
            next.delete(owner);
            return next;
        });
    }

    public open(idAttachment: string): void {
        this.activeId.set(idAttachment);
    }

    public close(): void {
        this.activeId.set(null);
    }

    public next(): void {
        this.step(1);
    }

    public previous(): void {
        this.step(-1);
    }

    private step(delta: number): void {
        const index = this.activeIndex();
        const images = this.images();
        if (index === null || images.length < 2) {
            return;
        }
        const target = (index + delta + images.length) % images.length;
        this.activeId.set(images[target].idAttachment);
    }
}

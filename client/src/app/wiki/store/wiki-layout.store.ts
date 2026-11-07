import { Injectable, computed, signal } from '@angular/core';

@Injectable({ providedIn: 'root' })
export class WikiLayoutStore {
    private static readonly fullWidthKey = 'wiki.fullWidth';
    private static readonly treeHiddenKey = 'wiki.treeHidden';
    private static readonly overlayQuery = '(max-width: 64rem)';

    private readonly fullWidthState = signal(this.readFlag(WikiLayoutStore.fullWidthKey));
    private readonly treeHiddenState = signal(this.readFlag(WikiLayoutStore.treeHiddenKey));
    private readonly overlayTreeOpen = signal(false);
    private readonly isOverlayState = signal(false);

    public readonly isFullWidth = this.fullWidthState.asReadonly();
    public readonly isOverlay = this.isOverlayState.asReadonly();
    public readonly isTreeHidden = computed(() =>
        this.isOverlayState() ? !this.overlayTreeOpen() : this.treeHiddenState()
    );

    public constructor() {
        const media =
            typeof window !== 'undefined' && typeof window.matchMedia === 'function'
                ? window.matchMedia(WikiLayoutStore.overlayQuery)
                : null;
        if (media) {
            this.isOverlayState.set(media.matches);
            media.addEventListener('change', event => {
                this.isOverlayState.set(event.matches);
                this.overlayTreeOpen.set(false);
            });
        }
    }

    public toggleFullWidth(): void {
        const isFullWidth = !this.fullWidthState();
        this.fullWidthState.set(isFullWidth);
        this.writeFlag(WikiLayoutStore.fullWidthKey, isFullWidth);
    }

    public toggleTree(): void {
        if (this.isOverlayState()) {
            this.overlayTreeOpen.update(isOpen => !isOpen);
            return;
        }
        const isTreeHidden = !this.treeHiddenState();
        this.treeHiddenState.set(isTreeHidden);
        this.writeFlag(WikiLayoutStore.treeHiddenKey, isTreeHidden);
    }

    public closeOverlayTree(): void {
        this.overlayTreeOpen.set(false);
    }

    private readFlag(key: string): boolean {
        try {
            return localStorage.getItem(key) === 'true';
        } catch {
            return false;
        }
    }

    private writeFlag(key: string, value: boolean): void {
        try {
            localStorage.setItem(key, String(value));
        } catch {
            return;
        }
    }
}

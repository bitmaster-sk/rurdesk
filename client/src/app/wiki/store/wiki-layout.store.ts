import { Injectable, computed, inject, signal } from '@angular/core';
import {
    SidePanelControl,
    SidePanelStore
} from 'src/app/ui/components/side-panel/side-panel.store';

/**
 * The wiki's reading width and its page tree. On a wide screen the tree is the shared secondary
 * panel and hides with it; below 64rem it is an overlay, closed until it is opened.
 */
@Injectable({ providedIn: 'root' })
export class WikiLayoutStore implements SidePanelControl {
    private static readonly fullWidthKey = 'wiki.fullWidth';
    private static readonly overlayQuery = '(max-width: 64rem)';

    private readonly panel = inject(SidePanelStore);

    private readonly fullWidthState = signal(this.readFlag(WikiLayoutStore.fullWidthKey));
    private readonly overlayTreeOpen = signal(false);
    private readonly isOverlayState = signal(false);

    public readonly isFullWidth = this.fullWidthState.asReadonly();
    public readonly isOverlay = this.isOverlayState.asReadonly();
    public readonly isHidden = computed(() =>
        this.isOverlayState() ? !this.overlayTreeOpen() : this.panel.isHidden()
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

    public hide(): void {
        if (this.isOverlayState()) {
            this.overlayTreeOpen.set(false);
            return;
        }
        this.panel.hide();
    }

    public show(): void {
        if (this.isOverlayState()) {
            this.overlayTreeOpen.set(true);
            return;
        }
        this.panel.show();
    }

    /** A navigation closes the overlay tree; the wide-screen panel stays as it is. */
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

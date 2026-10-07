import { Injectable, signal } from '@angular/core';

@Injectable({ providedIn: 'root' })
export class WikiLayoutStore {
    private static readonly fullWidthKey = 'wiki.fullWidth';
    private static readonly treeHiddenKey = 'wiki.treeHidden';

    private readonly fullWidthState = signal(this.readFlag(WikiLayoutStore.fullWidthKey));
    private readonly treeHiddenState = signal(this.readFlag(WikiLayoutStore.treeHiddenKey));

    public readonly isFullWidth = this.fullWidthState.asReadonly();
    public readonly isTreeHidden = this.treeHiddenState.asReadonly();

    public toggleFullWidth(): void {
        const isFullWidth = !this.fullWidthState();
        this.fullWidthState.set(isFullWidth);
        this.writeFlag(WikiLayoutStore.fullWidthKey, isFullWidth);
    }

    public toggleTree(): void {
        const isTreeHidden = !this.treeHiddenState();
        this.treeHiddenState.set(isTreeHidden);
        this.writeFlag(WikiLayoutStore.treeHiddenKey, isTreeHidden);
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

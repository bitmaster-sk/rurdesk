import { Injectable, Signal, signal } from '@angular/core';

const HIDDEN_KEY = 'sidePanelHidden';

/** Whether the secondary panel (Wiki, Messages, Agent chat) is hidden; one state for all pages, remembered across reloads. */
@Injectable({ providedIn: 'root' })
export class SidePanelStore {
    private readonly _isHidden = signal(SidePanelStore.readHidden());

    public readonly isHidden = this._isHidden.asReadonly();

    public hide(): void {
        this.write(true);
    }

    public show(): void {
        this.write(false);
    }

    private write(isHidden: boolean): void {
        this._isHidden.set(isHidden);
        try {
            localStorage.setItem(HIDDEN_KEY, isHidden ? '1' : '0');
        } catch {
            /* best-effort: the panel still hides for this visit */
        }
    }

    private static readHidden(): boolean {
        try {
            return localStorage.getItem(HIDDEN_KEY) === '1';
        } catch {
            return false;
        }
    }
}

/**
 * What the panel's hide button and `ui-side-panel-show` drive. `SidePanelStore` by default; a page
 * may provide its own, as the wiki does for its overlay on narrow screens.
 */
@Injectable({ providedIn: 'root', useExisting: SidePanelStore })
export abstract class SidePanelControl {
    public abstract readonly isHidden: Signal<boolean>;
    public abstract hide(): void;
    public abstract show(): void;
}

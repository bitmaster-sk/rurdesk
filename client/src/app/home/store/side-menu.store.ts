import { Injectable, signal } from '@angular/core';

const COLLAPSED_KEY = 'sideMenuCollapsed';

/** Whether the side menu is folded to an icon rail; remembered across reloads. */
@Injectable({ providedIn: 'root' })
export class SideMenuStore {
    private readonly _isCollapsed = signal(SideMenuStore.readCollapsed());

    public readonly isCollapsed = this._isCollapsed.asReadonly();

    public toggle(): void {
        const isCollapsed = !this._isCollapsed();
        this._isCollapsed.set(isCollapsed);
        try {
            localStorage.setItem(COLLAPSED_KEY, isCollapsed ? '1' : '0');
        } catch {
            /* best-effort: the rail still works for this visit */
        }
    }

    private static readCollapsed(): boolean {
        try {
            return localStorage.getItem(COLLAPSED_KEY) === '1';
        } catch {
            return false;
        }
    }
}

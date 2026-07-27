import { Injectable } from '@angular/core';

const KEY = 'rurdesk.command.recents';
// pre-rebrand key; migrated on first read, remove after a few releases
const LEGACY_KEY = 'robota.command.recents';
const CAP = 8;

@Injectable({ providedIn: 'root' })
export class RecentCommandsStore {
    public recentIds(): string[] {
        try {
            let raw = localStorage.getItem(KEY);
            if (raw === null) {
                raw = localStorage.getItem(LEGACY_KEY);
                if (raw !== null) {
                    localStorage.setItem(KEY, raw);
                    localStorage.removeItem(LEGACY_KEY);
                }
            }
            return raw ? (JSON.parse(raw) as string[]) : [];
        } catch {
            return [];
        }
    }

    public push(id: string): void {
        const next = [id, ...this.recentIds().filter(x => x !== id)].slice(0, CAP);
        try {
            localStorage.setItem(KEY, JSON.stringify(next));
        } catch {
            /* best-effort */
        }
    }
}

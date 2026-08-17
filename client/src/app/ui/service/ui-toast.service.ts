import { LiveAnnouncer } from '@angular/cdk/a11y';
import { Injectable, inject, signal } from '@angular/core';

export type UiToastSeverity = 'success' | 'info' | 'error';

/** A visible toast. `remaining`/timer state is NOT here — it lives in the service's
 *  internal timer map (mutating a field on an item in a `signal<readonly []>`
 *  would not change the array reference, so OnPush would not re-render). */
export interface UiToastItem {
    readonly id: number;
    readonly severity: UiToastSeverity;
    readonly detail: string;
    readonly life: number;
}

/** Per-toast timer bookkeeping. `paused` is the single source of truth driving
 *  both the JS auto-dismiss timeout and the CSS progress-bar `.is-paused` class. */
interface TimerEntry {
    handle: ReturnType<typeof setTimeout>;
    startedAt: number;
    remaining: number;
    paused: boolean;
}

const DEFAULT_LIFE_MS = 3000;

/**
 * Root store + lifecycle for product toasts (`ui-toast`). Holds the visible
 * list as a signal and owns
 * the auto-dismiss timers with hover/focus pause. Announces each toast through
 * CDK `LiveAnnouncer` (single SR channel — the visual node carries no ARIA role,
 * which would otherwise double-announce). No `ngOnDestroy`: it is a root
 * singleton that lives for the whole app; `dismiss` clears its own timer.
 */
@Injectable({ providedIn: 'root' })
export class UiToastService {
    /** Visible toasts, oldest first (rendered top-to-bottom → newest at the bottom). */
    public readonly toasts = signal<readonly UiToastItem[]>([]);
    /** Ids whose timer is currently paused; the host binds `[class.is-paused]` from this. */
    public readonly pausedIds = signal<ReadonlySet<number>>(new Set());

    private readonly liveAnnouncer = inject(LiveAnnouncer);
    private readonly timers = new Map<number, TimerEntry>();
    private idSeq = 0;

    public show(input: { severity: UiToastSeverity; detail: string; life?: number }): number {
        const id = ++this.idSeq;
        const life = input.life ?? DEFAULT_LIFE_MS;

        this.timers.set(id, {
            handle: setTimeout(() => this.dismiss(id), life),
            startedAt: Date.now(),
            remaining: life,
            paused: false
        });
        this.toasts.update(prev => [
            ...prev,
            { id, severity: input.severity, detail: input.detail, life }
        ]);

        // Single SR channel: LiveAnnouncer keeps a persistent hidden aria-live region,
        // reliable across VoiceOver/NVDA unlike role on a node inserted with its content.
        void this.liveAnnouncer.announce(
            input.detail,
            input.severity === 'error' ? 'assertive' : 'polite'
        );
        return id;
    }

    public dismiss(id: number): void {
        const entry = this.timers.get(id);
        if (!entry) {
            return; // already gone — idempotent, and guards hover/leave during leave-animation
        }
        clearTimeout(entry.handle);
        this.timers.delete(id);
        this.toasts.update(prev => prev.filter(t => t.id !== id));
        this.syncPaused();
    }

    /** Pause auto-dismiss (hover/focus). No-op if the toast is gone or already paused
     *  (a second pause would subtract elapsed time again and underflow `remaining`). */
    public pause(id: number): void {
        const entry = this.timers.get(id);
        if (!entry || entry.paused) {
            return;
        }
        clearTimeout(entry.handle);
        entry.remaining = Math.max(0, entry.remaining - (Date.now() - entry.startedAt));
        entry.paused = true;
        this.syncPaused();
    }

    /** Resume auto-dismiss (leave/blur). No-op if gone or not paused (a second resume
     *  would schedule a second timeout and orphan the first handle). */
    public resume(id: number): void {
        const entry = this.timers.get(id);
        if (!entry || !entry.paused) {
            return;
        }
        entry.startedAt = Date.now();
        entry.handle = setTimeout(() => this.dismiss(id), entry.remaining);
        entry.paused = false;
        this.syncPaused();
    }

    private syncPaused(): void {
        const paused = new Set<number>();
        for (const [id, entry] of this.timers) {
            if (entry.paused) {
                paused.add(id);
            }
        }
        this.pausedIds.set(paused);
    }
}

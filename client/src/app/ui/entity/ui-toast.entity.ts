export type UiToastSeverity = 'success' | 'info' | 'error';

/** A visible toast. `remaining`/timer state is NOT here — it lives in the service's
 *  internal timer map (mutating a field on an item in a `signal<readonly []>`
 *  would not change the array reference, so OnPush would not re-render). */
export interface UiToastAction {
    readonly label: string;
    readonly run: () => void;
}

export interface UiToastItem {
    readonly id: number;
    readonly severity: UiToastSeverity;
    readonly detail: string;
    readonly life: number;
    readonly action?: UiToastAction;
}

/** Per-toast timer bookkeeping. `paused` is the single source of truth driving
 *  both the JS auto-dismiss timeout and the CSS progress-bar `.is-paused` class. */
export interface TimerEntry {
    handle: ReturnType<typeof setTimeout>;
    startedAt: number;
    remaining: number;
    paused: boolean;
}

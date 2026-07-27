import { LiveAnnouncer } from '@angular/cdk/a11y';
import { Injector, runInInjectionContext } from '@angular/core';
import { UiToastService } from './ui-toast.service';

function build() {
    const announce = vi.fn();
    const injector = Injector.create({
        providers: [{ provide: LiveAnnouncer, useValue: { announce } }]
    });
    const service = runInInjectionContext(injector, () => new UiToastService());
    return { service, announce };
}

describe('UiToastService', () => {
    beforeEach(() => vi.useFakeTimers());
    afterEach(() => vi.useRealTimers());

    it('show enqueues a toast with the given fields and an id', () => {
        const { service, announce } = build();
        const id = service.show({ severity: 'error', detail: 'boom', life: 4000 });
        expect(service.toasts()).toEqual([{ id, severity: 'error', detail: 'boom', life: 4000 }]);
        expect(announce).toHaveBeenCalledWith('boom', 'assertive');
    });

    it('announces non-error toasts politely', () => {
        const { service, announce } = build();
        service.show({ severity: 'success', detail: 'ok' });
        expect(announce).toHaveBeenCalledWith('ok', 'polite');
    });

    it('auto-dismisses after life', () => {
        const { service } = build();
        service.show({ severity: 'info', detail: 'hi', life: 3000 });
        vi.advanceTimersByTime(2999);
        expect(service.toasts()).toHaveLength(1);
        vi.advanceTimersByTime(1);
        expect(service.toasts()).toHaveLength(0);
    });

    it('dismiss removes the specific toast and is idempotent', () => {
        const { service } = build();
        const a = service.show({ severity: 'info', detail: 'a', life: 5000 });
        service.show({ severity: 'info', detail: 'b', life: 5000 });
        service.dismiss(a);
        expect(service.toasts().map(t => t.detail)).toEqual(['b']);
        service.dismiss(a); // no throw, no change
        expect(service.toasts().map(t => t.detail)).toEqual(['b']);
    });

    it('pause stops the timer; resume finishes with the remaining time', () => {
        const { service } = build();
        const id = service.show({ severity: 'info', detail: 'x', life: 3000 });
        vi.advanceTimersByTime(1000);
        service.pause(id);
        expect(service.pausedIds().has(id)).toBe(true);
        vi.advanceTimersByTime(10000); // stays while paused
        expect(service.toasts()).toHaveLength(1);
        service.resume(id);
        expect(service.pausedIds().has(id)).toBe(false);
        vi.advanceTimersByTime(1999);
        expect(service.toasts()).toHaveLength(1);
        vi.advanceTimersByTime(1); // remaining 2000 elapsed
        expect(service.toasts()).toHaveLength(0);
    });

    it('double-pause does not underflow remaining (hover + focus)', () => {
        const { service } = build();
        const id = service.show({ severity: 'info', detail: 'x', life: 3000 });
        vi.advanceTimersByTime(1000);
        service.pause(id); // remaining -> 2000
        service.pause(id); // no-op (already paused) — must NOT subtract again
        service.resume(id);
        vi.advanceTimersByTime(1999);
        expect(service.toasts()).toHaveLength(1);
        vi.advanceTimersByTime(1);
        expect(service.toasts()).toHaveLength(0);
    });

    it('double-resume schedules only one timeout (no orphaned handle)', () => {
        const { service } = build();
        const id = service.show({ severity: 'info', detail: 'x', life: 3000 });
        service.pause(id);
        service.resume(id);
        service.resume(id); // no-op (not paused)
        // Exactly one 3000ms timeout is pending; it fires once and removes the toast.
        vi.advanceTimersByTime(3000);
        expect(service.toasts()).toHaveLength(0);
    });

    it('pause/resume after dismiss are no-ops (leaving-toast guard)', () => {
        const { service } = build();
        const id = service.show({ severity: 'info', detail: 'x', life: 3000 });
        service.dismiss(id);
        expect(() => {
            service.pause(id);
            service.resume(id);
        }).not.toThrow();
        vi.advanceTimersByTime(5000);
        expect(service.toasts()).toHaveLength(0); // no resurrection, no new timeout
    });

    it('runs independent timers for multiple toasts', () => {
        const { service } = build();
        service.show({ severity: 'info', detail: 'short', life: 1000 });
        service.show({ severity: 'info', detail: 'long', life: 5000 });
        vi.advanceTimersByTime(1000);
        expect(service.toasts().map(t => t.detail)).toEqual(['long']);
        vi.advanceTimersByTime(4000);
        expect(service.toasts()).toHaveLength(0);
    });
});

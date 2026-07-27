// @vitest-environment jsdom
import { Injector, NgZone, runInInjectionContext } from '@angular/core';
import { TranslateService } from '@ngx-translate/core';
import { GanttDragService } from './gantt-drag.service';
import { GanttTimelineService } from './gantt-timeline.service';

// GanttDragService injects GanttTimelineService + NgZone and attaches global DOM
// listeners on drag start, so it runs under jsdom with stubbed DI.
function buildService(): GanttDragService {
    const timelineInjector = Injector.create({
        providers: [{ provide: TranslateService, useValue: { instant: (k: string) => k } }]
    });
    const timeline = runInInjectionContext(timelineInjector, () => new GanttTimelineService());
    timeline.setRange(new Date('2026-04-01'), new Date('2026-04-30'));

    const injector = Injector.create({
        providers: [
            { provide: GanttTimelineService, useValue: timeline },
            { provide: NgZone, useValue: { runOutsideAngular: (fn: () => unknown) => fn() } }
        ]
    });
    return runInInjectionContext(injector, () => new GanttDragService());
}

describe('GanttDragService', () => {
    it('transitions from idle to moving on startMove', () => {
        const service = buildService();
        service.startMove(42, 100, new Date('2026-04-15T12:00:00'));

        expect(service.isDragging()).toBe(true);
    });

    it('transitions back to idle on cancel', () => {
        const service = buildService();
        service.startMove(42, 100, new Date('2026-04-15T12:00:00'));
        service.cancel();

        expect(service.isDragging()).toBe(false);
    });

    it('exposes a formatted tooltip for the current drag date', () => {
        const service = buildService();
        service.startMove(42, 100, new Date('2026-04-15T12:00:00'));

        expect(service.tooltipText()).toContain('Apr');
    });
});

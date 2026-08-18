// @vitest-environment jsdom
import { Injector, runInInjectionContext } from '@angular/core';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { GanttTimelineService } from './gantt-timeline.service';
import { GanttZoomLevel } from '../constants/gantt-zoom-config';

// GanttTimelineService injects I18nService and reads localStorage on init, so it
// runs under jsdom with a stubbed translate (instant returns the key). Constructed via
// `new` inside an injection context to avoid Angular DI JIT-compiling the @Injectable.
function buildService(): GanttTimelineService {
    const injector = Injector.create({
        providers: [{ provide: I18nService, useValue: { instant: (k: string) => k } }]
    });
    return runInInjectionContext(injector, () => new GanttTimelineService());
}

describe('GanttTimelineService', () => {
    let service: GanttTimelineService;

    beforeEach(() => {
        localStorage.clear();
        service = buildService();
        service.setRange(new Date('2026-04-01'), new Date('2026-04-30'));
    });

    it('toPixel and toDate are inverse operations', () => {
        const date = new Date('2026-04-15T10:00:00Z');
        const pixel = service.toPixel(date);
        const roundTrip = service.toDate(pixel);
        expect(Math.abs(roundTrip.getTime() - date.getTime())).toBeLessThan(1000);
    });

    it('snapToGrid snaps to week start at Week zoom', () => {
        service.setZoom(GanttZoomLevel.Week);
        const wednesday = new Date('2026-04-15T14:30:00Z'); // Wednesday
        const snapped = service.snapToGrid(wednesday);
        expect(snapped.getDay()).toBe(1); // Monday
    });

    it('snapToGrid snaps to day start at Day zoom', () => {
        service.setZoom(GanttZoomLevel.Day);
        const midday = new Date('2026-04-15T14:30:00Z');
        const snapped = service.snapToGrid(midday);
        expect(snapped.getHours()).toBe(0);
        expect(snapped.getMinutes()).toBe(0);
    });

    it('snapToNearest rounds down before the column midpoint', () => {
        service.setZoom(GanttZoomLevel.Day);
        const morning = new Date('2026-04-15T09:00:00');
        const snapped = service.snapToNearest(morning);
        expect(snapped.getDate()).toBe(15);
        expect(snapped.getHours()).toBe(0);
    });

    it('snapToNearest rounds up past the column midpoint', () => {
        service.setZoom(GanttZoomLevel.Day);
        const evening = new Date('2026-04-15T18:00:00');
        const snapped = service.snapToNearest(evening);
        expect(snapped.getDate()).toBe(16);
        expect(snapped.getHours()).toBe(0);
    });

    it('snapToNearest rounds up past midweek at Week zoom', () => {
        service.setZoom(GanttZoomLevel.Week);
        const friday = new Date('2026-04-17T12:00:00'); // Friday, past mid-week
        const snapped = service.snapToNearest(friday);
        expect(snapped.getDay()).toBe(1); // Monday
        expect(snapped.getDate()).toBe(20); // next week's Monday
    });

    it('getColumns produces correct count for Day zoom', () => {
        service.setZoom(GanttZoomLevel.Day);
        const columns = service.getColumns();
        expect(columns.length).toBeGreaterThanOrEqual(29);
    });
});

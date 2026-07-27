import { describe, expect, it } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { Component } from '@angular/core';
import { GanttWbsPanelComponent } from './gantt-wbs-panel';
import { GanttTimelineService } from '../../service/gantt-timeline.service';

// buildReorder is pure — no DOM needed. Neighbours are derived server-side, so
// only { movedId, order } is emitted.
describe('GanttWbsPanelComponent.buildReorder', () => {
    it('moves item to the end', () => {
        const result = GanttWbsPanelComponent.buildReorder([10, 20, 30], 0, 2);
        expect(result).toEqual({ movedId: 10, order: [20, 30, 10] });
    });

    it('moves item into the middle', () => {
        const result = GanttWbsPanelComponent.buildReorder([10, 20, 30], 2, 1);
        expect(result).toEqual({ movedId: 30, order: [10, 30, 20] });
    });
});

// Empty-template override so no child components/CVAs render.
@Component({ selector: 'app-gantt-wbs-panel', template: '', standalone: false })
class StubWbsPanel extends GanttWbsPanelComponent {}

describe('GanttWbsPanelComponent — click-vs-drag guard', () => {
    it('suppresses the click that fires right after a drag, then re-enables clicks', async () => {
        TestBed.configureTestingModule({
            declarations: [StubWbsPanel],
            providers: [{ provide: GanttTimelineService, useValue: { rowHeight: () => 40 } }]
        });
        const fixture = TestBed.createComponent(StubWbsPanel);
        const cmp = fixture.componentInstance;
        const clicks: unknown[] = [];
        cmp.taskClicked.subscribe(e => clicks.push(e));

        cmp.onRowDragStarted();
        cmp.onTaskClick(10, false);
        expect(clicks).toEqual([]); // click during/after drag swallowed

        cmp.onRowDragEnded();
        await new Promise(r => setTimeout(r)); // guard clears on next tick
        cmp.onTaskClick(10, false);
        expect(clicks.length).toBe(1); // normal clicks work again
    });
});

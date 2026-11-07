import { describe, expect, it } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { Component } from '@angular/core';
import { GanttWbsPanelComponent } from './gantt-wbs-panel';
import { GanttTimelineService } from '../../service/gantt-timeline.service';

describe('GanttWbsPanelComponent.resolveDrop', () => {
    const parents = new Map([
        [2, 1],
        [3, 2],
        [5, 2]
    ]);
    const order = [1, 2, 3, 5, 4];

    it('drops between rows near the edges and into a row in the middle', () => {
        expect(GanttWbsPanelComponent.resolveDrop(4, 1, 0.1, 0, parents, order)).toMatchObject({
            kind: 'before',
            isAllowed: true
        });
        expect(GanttWbsPanelComponent.resolveDrop(4, 1, 0.5, 0, parents, order)).toMatchObject({
            kind: 'into',
            isAllowed: true
        });
        expect(GanttWbsPanelComponent.resolveDrop(4, 1, 0.9, 0, parents, order)).toMatchObject({
            kind: 'after',
            isAllowed: true
        });
    });

    it('refuses a drop into the own subtree', () => {
        expect(GanttWbsPanelComponent.resolveDrop(1, 3, 0.5, 0, parents, order).isAllowed).toBe(
            false
        );
    });

    it('turns a sideways drag on the own row into indent or outdent', () => {
        expect(GanttWbsPanelComponent.resolveDrop(5, 5, 0.5, 30, parents, order)).toMatchObject({
            kind: 'indent',
            isAllowed: true
        });
        expect(GanttWbsPanelComponent.resolveDrop(3, 3, 0.5, -30, parents, order)).toMatchObject({
            kind: 'outdent',
            isAllowed: true
        });
        expect(GanttWbsPanelComponent.resolveDrop(3, 3, 0.5, 5, parents, order).isAllowed).toBe(
            false
        );
    });

    it('refuses indent without a previous sibling and outdent of a root task', () => {
        expect(GanttWbsPanelComponent.resolveDrop(3, 3, 0.5, 30, parents, order)).toMatchObject({
            kind: 'indent',
            isAllowed: false
        });
        expect(GanttWbsPanelComponent.resolveDrop(1, 1, 0.5, -30, parents, order)).toMatchObject({
            kind: 'outdent',
            isAllowed: false
        });
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

        cmp.onRowDragStarted(10);
        cmp.onTaskClick(10, false);
        expect(clicks).toEqual([]); // click during/after drag swallowed

        cmp.onRowDragEnded();
        await new Promise(r => setTimeout(r)); // guard clears on next tick
        cmp.onTaskClick(10, false);
        expect(clicks.length).toBe(1); // normal clicks work again
    });
});

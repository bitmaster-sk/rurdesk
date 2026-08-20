import { describe, it, expect, beforeEach } from 'vitest';
import { userEvent } from 'vitest/browser';
import { TestBed } from '@angular/core/testing';
import { Component, input, output } from '@angular/core';
import { provideRouter } from '@angular/router';

import { GanttTimelineBodyComponent } from './gantt-timeline-body';
import { UiTooltipComponent } from '../../../../../ui/components/tooltip/tooltip.component';
import { GanttTimelineService } from '../../service/gantt-timeline.service';
import { ScheduledIssue } from '../../../../model/extended-issue.model';

@Component({ selector: 'app-gantt-timeline-header', template: '', standalone: true })
class TimelineHeaderStub {
    public readonly headerRows = input<any[]>([]);
    public readonly columns = input<any[]>([]);
    public readonly totalWidth = input<number>(0);
}

@Component({ selector: 'app-gantt-task-bar', template: '', standalone: true })
class TaskBarStub {
    public readonly task = input.required<any>();
    public readonly cardMode = input.required<any>();
    public readonly rowIndex = input.required<number>();
    public readonly isOnCriticalPath = input<boolean>(false);
    public readonly isDimmed = input<boolean>(false);
    public readonly relationDropSide = input<any>(null);
    public readonly isSelected = input<boolean>(false);
    public readonly cascadeSlideDelayMs = input<number | null>(null);
    public readonly hovered = output<number | null>();
    public readonly dragStarted = output<any>();
    public readonly resizeEnded = output<any>();
    public readonly contextMenu = output<any>();
    public readonly connectionDragStarted = output<any>();
}

@Component({ selector: 'app-gantt-arrow-layer', template: '', standalone: true })
class ArrowLayerStub {
    public readonly tasks = input<any[]>([]);
    public readonly relations = input<any[]>([]);
    public readonly totalWidth = input<number>(0);
    public readonly totalHeight = input<number>(0);
    public readonly scrollLeft = input<number>(0);
    public readonly scrollTop = input<number>(0);
    public readonly viewportWidth = input<number>(0);
    public readonly viewportHeight = input<number>(0);
    public readonly selectedRelationId = input<number | null>(null);
    public readonly criticalRelationIds = input<any>(new Set());
    public readonly criticalRelationOrder = input<any>(new Map());
    public readonly isCriticalTracing = input<boolean>(false);
    public readonly isCriticalPathEnabled = input<boolean>(false);
    public readonly drawingLine = input<any>(null);
    public readonly dropTarget = input<any>(null);
    public readonly drawInRelation = input<any>(null);
    public readonly isSettling = input<boolean>(false);
    public readonly arrowClicked = output<number>();
    public readonly deleteRequested = output<any>();
}

const timelineServiceMock = {
    rowHeight: () => 64,
    getTotalWidth: () => 5000,
    getTodayPixel: () => 0
};

function makeTask(idIssuePublic: number): ScheduledIssue {
    return {
        idIssue: idIssuePublic,
        idIssuePublic,
        idProject: 10,
        idState: null,
        idSeverity: null,
        title: `Task ${idIssuePublic}`,
        description: '',
        tracked: 0,
        estimated: 3600,
        scheduledAt: new Date('2025-01-15T00:00:00Z'),
        state: undefined,
        severity: undefined,
        assignedToUser: undefined
    };
}

describe('GanttTimelineBodyComponent (TestBed)', () => {
    let fixture: any;
    let scroller: HTMLElement;

    beforeEach(async () => {
        TestBed.configureTestingModule({
            imports: [TimelineHeaderStub, TaskBarStub, ArrowLayerStub],
            declarations: [GanttTimelineBodyComponent, UiTooltipComponent],
            providers: [
                { provide: GanttTimelineService, useValue: timelineServiceMock },
                provideRouter([])
            ]
        });
        await TestBed.compileComponents();
        fixture = TestBed.createComponent(GanttTimelineBodyComponent);
        fixture.componentRef.setInput('tasks', [makeTask(1), makeTask(2)]);
        fixture.componentRef.setInput('columns', []);
        fixture.componentRef.setInput('headerRows', []);
        fixture.componentRef.setInput('cardMode', 'GanttCompact');
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        scroller = fixture.nativeElement.querySelector('.gantt-body');
        // Chrome only offers a scroll container as a tab stop once it actually overflows,
        // so bound the box explicitly instead of relying on the ambient fixture layout.
        scroller.style.width = '300px';
        scroller.style.height = '200px';
    });

    describe('keyboard tab order', () => {
        it('never lands focus on the timeline scroller while tabbing', async () => {
            expect(scroller.scrollWidth).toBeGreaterThan(scroller.clientWidth);

            for (let i = 0; i < 5; i++) {
                await userEvent.tab();
                expect(document.activeElement?.closest('.gantt-body')).toBeNull();
            }
        });

        it('draws no focus ring on the scroller when it gets focused anyway', () => {
            scroller.focus();

            expect(document.activeElement).toBe(scroller);
            expect(getComputedStyle(scroller).outlineStyle).toBe('none');
        });
    });

    describe('drag date tooltip', () => {
        const ghost = (top: number) => [
            { taskId: 1, left: 120, top, width: 80, tooltipText: 'Wed 15 Jan, 09:00' }
        ];

        function bubble(): HTMLElement | null {
            return document.querySelector('.cdk-overlay-container .ui-tooltip');
        }

        function centerX(rect: DOMRect): number {
            return rect.left + rect.width / 2;
        }

        async function setGhostBars(bars: unknown[]): Promise<void> {
            fixture.componentRef.setInput('ghostBars', bars);
            fixture.detectChanges();
            await fixture.whenStable();
        }

        it('renders the date bubble outside the timeline scroller so bars cannot cover it', async () => {
            await setGhostBars(ghost(64 + 2));

            const tip = bubble();
            expect(tip).not.toBeNull();
            expect(tip!.textContent?.trim()).toBe('Wed 15 Jan, 09:00');
            expect(tip!.closest('.gantt-body')).toBeNull();
        });

        it('places the bubble horizontally over the ghost bar', async () => {
            await setGhostBars(ghost(64 + 2));

            const canvas = fixture.nativeElement.querySelector('.gantt-body--canvas');
            const ghostBar = fixture.nativeElement.querySelector('.gantt-body--ghost');
            const tipRect = bubble()!.getBoundingClientRect();
            const barRect = ghostBar.getBoundingClientRect();

            expect(canvas).not.toBeNull();
            expect(Math.abs(centerX(tipRect) - centerX(barRect))).toBeLessThan(24);
        });

        it('keeps the bubble inside the viewport when the ghost sits in the first row', async () => {
            await setGhostBars(ghost(2));

            const tipRect = bubble()!.getBoundingClientRect();

            expect(tipRect.top).toBeGreaterThanOrEqual(0);
            expect(tipRect.bottom).toBeLessThanOrEqual(window.innerHeight);
        });

        it('removes the bubble when the drag ends', async () => {
            await setGhostBars(ghost(64 + 2));
            expect(bubble()).not.toBeNull();

            await setGhostBars([]);

            expect(bubble()).toBeNull();
        });
    });

    describe('scroll state', () => {
        it('publishes viewport and scroll offsets on scroll', () => {
            scroller.scrollLeft = 120;
            const emitted: unknown[] = [];
            fixture.componentInstance.scrolled.subscribe((e: unknown) => emitted.push(e));

            fixture.componentInstance.onScroll();

            expect(fixture.componentInstance.scrollLeft()).toBe(120);
            expect(fixture.componentInstance.viewportWidth()).toBe(scroller.clientWidth);
            expect(emitted).toEqual([{ scrollTop: 0, scrollLeft: 120 }]);
        });
    });
});

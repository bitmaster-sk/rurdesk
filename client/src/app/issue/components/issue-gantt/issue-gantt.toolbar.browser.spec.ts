import { describe, it, expect, vi, beforeEach } from 'vitest';
import { GanttZoomLevel } from './constants/gantt-zoom-config';
import { STORAGE_KEY_CARD_MODE, STORAGE_KEY_MINIMAP } from './constants/gantt-storage-keys';
import { createGanttFixture } from './gantt-testbed.helper';

describe('IssueGanttComponent toolbar handlers (TestBed)', () => {
    let comp: any;
    let mocks: any;

    beforeEach(async () => {
        localStorage.clear();
        const result = await createGanttFixture();
        comp = result.comp;
        mocks = result.mocks;
    });

    // =========================================================================
    // onToggleFilter
    // =========================================================================

    it('delegates to issueFilterStore.toggleShowFilter', () => {
        comp.onToggleFilter();
        expect(mocks.issueFilterStoreMock.toggleShowFilter).toHaveBeenCalled();
    });

    // =========================================================================
    // onZoomChange
    // =========================================================================

    it('clears cascadeSlide and sets zoom level', () => {
        comp.onZoomChange(GanttZoomLevel.Day);
        expect(mocks.timelineServiceMock.setZoom).toHaveBeenCalledWith(GanttZoomLevel.Day);
    });

    // =========================================================================
    // onCardModeChange
    // =========================================================================

    describe('onCardModeChange', () => {
        it('sets cardMode signal and persists to localStorage', () => {
            comp.onCardModeChange('GanttCompact');
            expect(comp.cardMode()).toBe('GanttCompact');
            expect(localStorage.getItem(STORAGE_KEY_CARD_MODE)).toBe('GanttCompact');
        });

        it('comfortable mode sets row height to 72', () => {
            mocks.timelineServiceMock.rowHeight.set.mockClear();
            comp.onCardModeChange('GanttComfort');
            expect(mocks.timelineServiceMock.rowHeight.set).toHaveBeenCalledWith(72);
        });

        it('compact mode sets row height to 38', () => {
            mocks.timelineServiceMock.rowHeight.set.mockClear();
            comp.onCardModeChange('GanttCompact');
            expect(mocks.timelineServiceMock.rowHeight.set).toHaveBeenCalledWith(38);
        });
    });

    // =========================================================================
    // onToggleWbs
    // =========================================================================

    describe('onToggleWbs', () => {
        it('toggles isWbsCollapsed from false to true', () => {
            expect(comp.isWbsCollapsed()).toBe(false);
            comp.onToggleWbs();
            expect(comp.isWbsCollapsed()).toBe(true);
        });

        it('toggles isWbsCollapsed from true to false', () => {
            comp.isWbsCollapsed.set(true);
            comp.onToggleWbs();
            expect(comp.isWbsCollapsed()).toBe(false);
        });
    });

    // =========================================================================
    // onToggleMinimap
    // =========================================================================

    describe('onToggleMinimap', () => {
        it('toggles from visible to hidden, persists to localStorage', () => {
            expect(comp.isMinimapVisible()).toBe(true);
            comp.onToggleMinimap();
            expect(comp.isMinimapVisible()).toBe(false);
            expect(localStorage.getItem(STORAGE_KEY_MINIMAP)).toBe('false');
        });

        it('toggles from hidden to visible', () => {
            comp.isMinimapVisible.set(false);
            comp.onToggleMinimap();
            expect(comp.isMinimapVisible()).toBe(true);
            expect(localStorage.getItem(STORAGE_KEY_MINIMAP)).toBe('true');
        });
    });

    // =========================================================================
    // onScrollToToday
    // =========================================================================

    it('onScrollToToday delegates to timelineBodyRef.scrollToToday', () => {
        expect(() => comp.onScrollToToday()).not.toThrow();
    });

    // =========================================================================
    // onToggleCriticalPath
    // =========================================================================

    describe('onToggleCriticalPath', () => {
        beforeEach(() => vi.useFakeTimers());
        afterEach(() => vi.useRealTimers());

        it('enabling: sets isCriticalPathEnabled and isCriticalTracing', () => {
            comp.onToggleCriticalPath();
            expect(comp.isCriticalPathEnabled()).toBe(true);
            expect(comp.isCriticalTracing()).toBe(true);
        });

        it('enabling: timer duration = segments * 120 + 700', () => {
            // No relations in default setup → 0 segments → 700ms
            comp.isCriticalPathEnabled.set(false);
            comp.isCriticalTracing.set(false);
            comp.onToggleCriticalPath();
            vi.advanceTimersByTime(699);
            expect(comp.isCriticalTracing()).toBe(true);
            vi.advanceTimersByTime(1);
            expect(comp.isCriticalTracing()).toBe(false);
        });

        it('disabling: sets isCriticalPathEnabled false and isCriticalTracing false', () => {
            comp.isCriticalPathEnabled.set(true);
            comp.onToggleCriticalPath();
            expect(comp.isCriticalPathEnabled()).toBe(false);
            expect(comp.isCriticalTracing()).toBe(false);
        });

        it('disabling: clears existing trace timer (no leftover tracing set)', () => {
            comp.isCriticalPathEnabled.set(true);
            comp.isCriticalTracing.set(true);
            comp.onToggleCriticalPath();
            expect(comp.isCriticalTracing()).toBe(false);
            vi.advanceTimersByTime(5000);
            expect(comp.isCriticalTracing()).toBe(false);
        });
    });

    // =========================================================================
    // onWbsScrolled
    // =========================================================================

    it('onWbsScrolled delegates to timelineBodyRef.syncScrollFrom', () => {
        expect(() => comp.onWbsScrolled(150)).not.toThrow();
    });

    // =========================================================================
    // onTimelineScrolled
    // =========================================================================

    it('onTimelineScrolled sets viewport signals', () => {
        comp.wbsPanelRef = () => ({ syncScrollFrom: vi.fn() }) as any;
        expect(() => comp.onTimelineScrolled({ scrollTop: 100, scrollLeft: 200 })).not.toThrow();
        expect(comp.viewportScrollLeft()).toBe(200);
    });

    // =========================================================================
    // onTaskHovered
    // =========================================================================

    it('sets hoveredTaskId signal', () => {
        comp.onTaskHovered(42);
        expect(comp.hoveredTaskId()).toBe(42);
    });

    it('clears hoveredTaskId with null', () => {
        comp.onTaskHovered(42);
        comp.onTaskHovered(null);
        expect(comp.hoveredTaskId()).toBeNull();
    });

    // =========================================================================
    // onBarContextMenu
    // =========================================================================

    it('with valid issue: shows quick actions', () => {
        expect(() =>
            comp.onBarContextMenu({ taskId: 1, event: { clientX: 0, clientY: 0 } as any })
        ).not.toThrow();
    });

    it('without issue in map: no-op', () => {
        expect(() =>
            comp.onBarContextMenu({ taskId: 99, event: { clientX: 0, clientY: 0 } as any })
        ).not.toThrow();
    });

    // =========================================================================
    // onWbsTaskClicked
    // =========================================================================

    it('scheduled task: scrolls timeline to task position', () => {
        expect(() => comp.onWbsTaskClicked({ taskId: 1, isBacklog: false })).not.toThrow();
    });

    it('backlog task: no-op', () => {
        expect(() => comp.onWbsTaskClicked({ taskId: 5, isBacklog: true })).not.toThrow();
    });

    it('scheduled task not found: no-op', () => {
        expect(() => comp.onWbsTaskClicked({ taskId: 99, isBacklog: false })).not.toThrow();
    });

    // =========================================================================
    // onBarDragStarted
    // =========================================================================

    it('with task having scheduledAt: calls dragService.startMove', () => {
        expect(() =>
            comp.onBarDragStarted({ taskId: 1, event: { clientX: 100 } as any })
        ).not.toThrow();
        expect(mocks.dragServiceMock.startMove).toHaveBeenCalled();
    });

    it('without task in map: no-op', () => {
        mocks.dragServiceMock.startMove.mockClear();
        comp.onBarDragStarted({ taskId: 99, event: { clientX: 100 } as any });
        expect(mocks.dragServiceMock.startMove).not.toHaveBeenCalled();
    });

    // =========================================================================
    // onBacklogDragStarted
    // =========================================================================

    it('sets canvas offset and starts backlog schedule', () => {
        expect(() =>
            comp.onBacklogDragStarted({ taskId: 5, event: { clientX: 200 } as any })
        ).not.toThrow();
        expect(mocks.dragServiceMock.startBacklogSchedule).toHaveBeenCalledWith(5, 200);
    });

    // =========================================================================
    // onConnectionHandleDragStarted
    // =========================================================================

    it('calls dragService.startRelationDraw with correct args', () => {
        comp.onConnectionHandleDragStarted({
            taskId: 1,
            side: 'right' as any,
            event: { clientX: 100, clientY: 200 } as any
        });
        expect(mocks.dragServiceMock.startRelationDraw).toHaveBeenCalledWith(1, 'right', 100, 200);
    });

    // =========================================================================
    // onLoadMoreBacklog
    // =========================================================================

    it('delegates to ganttService.loadMoreBacklog', () => {
        comp.onLoadMoreBacklog();
        expect(mocks.ganttServiceMock.loadMoreBacklog).toHaveBeenCalled();
    });

    // =========================================================================
    // onWheel (cursor-anchored zoom)
    // =========================================================================

    describe('onWheel', () => {
        function makeWheelEvent(over: any = {}): any {
            return {
                ctrlKey: false,
                deltaY: 0,
                clientX: 0,
                preventDefault: vi.fn(),
                ...over
            };
        }

        it('without ctrlKey: no-op', () => {
            const event = makeWheelEvent({ ctrlKey: false });
            comp.onWheel(event);
            expect(event.preventDefault).not.toHaveBeenCalled();
            expect(mocks.timelineServiceMock.setZoom).not.toHaveBeenCalled();
        });

        it('ctrl+wheel down (deltaY > 0): zooms out', () => {
            mocks.timelineServiceMock.setZoom.mockClear();
            const event = makeWheelEvent({ ctrlKey: true, deltaY: 100, clientX: 500 });
            comp.onWheel(event);
            expect(event.preventDefault).toHaveBeenCalled();
            expect(mocks.timelineServiceMock.setZoom).toHaveBeenCalledWith(GanttZoomLevel.Month);
        });

        it('ctrl+wheel up (deltaY < 0): zooms in', () => {
            mocks.timelineServiceMock.setZoom.mockClear();
            const event = makeWheelEvent({ ctrlKey: true, deltaY: -100, clientX: 500 });
            comp.onWheel(event);
            expect(mocks.timelineServiceMock.setZoom).toHaveBeenCalledWith(GanttZoomLevel.Day);
        });

        it('without container (null timelineBodyRef): still zooms', () => {
            mocks.timelineServiceMock.setZoom.mockClear();
            const event = makeWheelEvent({ ctrlKey: true, deltaY: 100, clientX: 500 });
            expect(() => comp.onWheel(event)).not.toThrow();
            expect(mocks.timelineServiceMock.setZoom).toHaveBeenCalledWith(GanttZoomLevel.Month);
        });

        it('zoom out at last level: no zoom change', () => {
            mocks.timelineServiceMock.zoomLevel.mockReturnValue(GanttZoomLevel.Month);
            mocks.timelineServiceMock.setZoom.mockClear();
            const event = makeWheelEvent({ ctrlKey: true, deltaY: 100, clientX: 500 });
            comp.onWheel(event);
            expect(mocks.timelineServiceMock.setZoom).not.toHaveBeenCalled();
        });

        it('cursor-anchored: adjusts scrollLeft so the date under cursor stays put', () => {
            const dateUnderCursor = new Date('2025-01-15T00:00:00Z');
            const newPixel = 800;

            const scrollContainer = {
                getBoundingClientRect: () => ({ left: 100, right: 1100, top: 0, bottom: 500 }),
                scrollLeft: 200,
                clientWidth: 1000
            };
            let capturedScrollLeft = 0;
            Object.defineProperty(scrollContainer, 'scrollLeft', {
                get: () => 200,
                set: (v: number) => {
                    capturedScrollLeft = v;
                },
                configurable: true
            });

            mocks.timelineServiceMock.toDate.mockReturnValue(dateUnderCursor);
            mocks.timelineServiceMock.toPixel.mockReturnValue(newPixel);
            comp.timelineBodyRef = () => ({ getScrollContainer: () => scrollContainer }) as any;

            const event = makeWheelEvent({ ctrlKey: true, deltaY: 100, clientX: 400 });
            comp.onWheel(event);

            // cursorX = 400 - 100 (rect.left) + 200 (scrollLeft) = 500
            // cursorOffset = 400 - 100 = 300
            // scrollLeft = newPixel(800) - cursorOffset(300) = 500
            expect(capturedScrollLeft).toBe(500);
        });
    });
});

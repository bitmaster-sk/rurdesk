import { describe, it, expect, vi, beforeEach } from 'vitest';
import { GanttZoomLevel } from './constants/gantt-zoom-config';
import { createGanttFixture } from './gantt-testbed.helper';
import { IssueGanttComponent } from './issue-gantt.component';
import { TestBed } from '@angular/core/testing';
import { IssueRelationType } from '../../constants/issue-relation-type.enum';
import { ReadIssueRelationDto } from '../../model/issue-relation.model';

describe('IssueGanttComponent keyboard navigation (TestBed)', () => {
    let fixture: any;
    let comp: IssueGanttComponent;
    let mocks: any;

    beforeEach(async () => {
        localStorage.clear();
        const result = await createGanttFixture();
        fixture = result.fixture;
        comp = result.comp;
        mocks = result.mocks;
    });

    function keyDown(key: string, target: EventTarget | null = null) {
        const event = new KeyboardEvent('keydown', { key, bubbles: true });
        if (target) Object.defineProperty(event, 'target', { value: target });
        fixture.nativeElement.dispatchEvent(event);
        fixture.detectChanges();
    }

    // =========================================================================
    // Gate / editable target
    // =========================================================================

    it('ignores all keys while the command palette overlay is open', () => {
        mocks.commandPaletteMock.isOverlayOpen.mockReturnValue(true);
        keyDown('ArrowDown');
        expect(comp.selectedTaskIndex()).toBeNull();
    });

    it('ignores keys when the event target is an input element', () => {
        const input = document.createElement('input');
        keyDown('ArrowDown', input);
        expect(comp.selectedTaskIndex()).toBeNull();
    });

    it('ignores keys when the event target is a textarea element', () => {
        const textarea = document.createElement('textarea');
        keyDown('ArrowDown', textarea);
        expect(comp.selectedTaskIndex()).toBeNull();
    });

    it('ignores keys when the event target is inside a contenteditable element', () => {
        const div = document.createElement('div');
        div.contentEditable = 'true';
        keyDown('ArrowDown', div);
        expect(comp.selectedTaskIndex()).toBeNull();
    });

    // =========================================================================
    // ArrowDown
    // =========================================================================

    describe('ArrowDown', () => {
        it('selects first task from null selection', () => {
            keyDown('ArrowDown');
            expect(comp.selectedTaskIndex()).toBe(0);
        });

        it('advances selection by one', () => {
            comp.selectedTaskIndex.set(1);
            keyDown('ArrowDown');
            expect(comp.selectedTaskIndex()).toBe(2);
        });

        it('clamps at last task', () => {
            comp.selectedTaskIndex.set(2);
            keyDown('ArrowDown');
            expect(comp.selectedTaskIndex()).toBe(2);
        });
    });

    // =========================================================================
    // ArrowUp
    // =========================================================================

    describe('ArrowUp', () => {
        it('selects first task from null selection', () => {
            keyDown('ArrowUp');
            expect(comp.selectedTaskIndex()).toBe(0);
        });

        it('decrements selection by one', () => {
            comp.selectedTaskIndex.set(2);
            keyDown('ArrowUp');
            expect(comp.selectedTaskIndex()).toBe(1);
        });

        it('clamps at 0', () => {
            comp.selectedTaskIndex.set(0);
            keyDown('ArrowUp');
            expect(comp.selectedTaskIndex()).toBe(0);
        });
    });

    // =========================================================================
    // ArrowLeft / ArrowRight
    // =========================================================================

    it('ArrowLeft does not change selection', () => {
        keyDown('ArrowLeft');
        expect(comp.selectedTaskIndex()).toBeNull();
    });

    it('ArrowRight does not change selection', () => {
        keyDown('ArrowRight');
        expect(comp.selectedTaskIndex()).toBeNull();
    });

    // =========================================================================
    // t / Home (scroll to today)
    // =========================================================================

    it('t scrolls to today', () => {
        keyDown('t');
        expect(comp.selectedTaskIndex()).toBeNull();
    });

    it('Home scrolls to today', () => {
        keyDown('Home');
        expect(comp.selectedTaskIndex()).toBeNull();
    });

    // =========================================================================
    // + / = / - (zoom)
    // =========================================================================

    it('+ zooms in', () => {
        keyDown('+');
        expect(mocks.timelineServiceMock.setZoom).toHaveBeenCalledWith(GanttZoomLevel.Day);
    });

    it('= zooms in', () => {
        keyDown('=');
        expect(mocks.timelineServiceMock.setZoom).toHaveBeenCalledWith(GanttZoomLevel.Day);
    });

    it('- zooms out', () => {
        keyDown('-');
        expect(mocks.timelineServiceMock.setZoom).toHaveBeenCalledWith(GanttZoomLevel.Month);
    });

    it('+ on first zoom level is a no-op', () => {
        mocks.timelineServiceMock.zoomLevel.mockReturnValue(GanttZoomLevel.Hour);
        mocks.timelineServiceMock.setZoom.mockClear();
        keyDown('+');
        expect(mocks.timelineServiceMock.setZoom).not.toHaveBeenCalled();
    });

    it('- on last zoom level is a no-op', () => {
        mocks.timelineServiceMock.zoomLevel.mockReturnValue(GanttZoomLevel.Month);
        mocks.timelineServiceMock.setZoom.mockClear();
        keyDown('-');
        expect(mocks.timelineServiceMock.setZoom).not.toHaveBeenCalled();
    });

    // =========================================================================
    // Enter
    // =========================================================================

    it('Enter navigates to the selected issue detail', () => {
        comp.selectedTaskIndex.set(0);
        expect(() => keyDown('Enter')).not.toThrow();
    });

    it('Enter with null selection does nothing', () => {
        expect(() => keyDown('Enter')).not.toThrow();
    });

    it('Enter with out-of-range index does nothing', () => {
        comp.selectedTaskIndex.set(99);
        expect(() => keyDown('Enter')).not.toThrow();
    });

    // =========================================================================
    // Delete / Backspace
    // =========================================================================

    it('Delete with null selectedRelationId does nothing', () => {
        comp.selectedRelationId.set(null);
        expect(() => keyDown('Delete')).not.toThrow();
    });

    it('Delete with selected relation but no matching id does nothing', () => {
        comp.selectedRelationId.set(99);
        expect(() => keyDown('Delete')).not.toThrow();
    });

    it('Backspace clears selectedRelationId on null relation', () => {
        comp.selectedRelationId.set(null);
        expect(() => keyDown('Backspace')).not.toThrow();
    });

    // =========================================================================
    // Escape
    // =========================================================================

    it('Escape clears selection', () => {
        comp.selectedTaskIndex.set(1);
        comp.selectedRelationId.set(5);
        keyDown('Escape');
        expect(comp.selectedTaskIndex()).toBeNull();
        expect(comp.selectedRelationId()).toBeNull();
    });

    it('Escape cancels an ongoing drag', () => {
        mocks.dragServiceMock.isDragging.mockReturnValue(true);
        keyDown('Escape');
        expect(mocks.dragServiceMock.cancel).toHaveBeenCalled();
    });

    it('Escape does not cancel when no drag is active', () => {
        mocks.dragServiceMock.isDragging.mockReturnValue(false);
        mocks.dragServiceMock.cancel.mockClear();
        keyDown('Escape');
        expect(mocks.dragServiceMock.cancel).not.toHaveBeenCalled();
    });

    // =========================================================================
    // Unmapped keys
    // =========================================================================

    it('unmapped key does not throw', () => {
        expect(() => keyDown('x')).not.toThrow();
    });
});

// =========================================================================
// Empty task list
// =========================================================================

describe('IssueGanttComponent empty task list (TestBed)', () => {
    it('ArrowDown on empty task list stays at -1', async () => {
        localStorage.clear();
        const { fixture, comp } = await createGanttFixture({ tasks: [] });
        fixture.nativeElement.dispatchEvent(
            new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true })
        );
        fixture.detectChanges();
        expect(comp.selectedTaskIndex()).toBe(-1);
    });
});

// =========================================================================
// Delete / Backspace with outbound relation
// =========================================================================

describe('IssueGanttComponent Delete with relations (TestBed)', () => {
    let fixture: any;
    let comp: any;

    beforeEach(async () => {
        localStorage.clear();
        const relations: ReadIssueRelationDto[] = [
            {
                idIssueRelation: 10,
                direction: 'outbound',
                relationType: IssueRelationType.Schedule,
                from: { idIssuePublic: 1 },
                to: { idIssuePublic: 2 }
            } as any
        ];
        const result = await createGanttFixture({ relations });
        fixture = result.fixture;
        comp = result.comp;
    });

    function keyDown(key: string) {
        fixture.nativeElement.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true }));
        fixture.detectChanges();
    }

    it('Delete on a selected outbound relation calls onDeleteRelation', () => {
        const spy = vi.spyOn(comp, 'onDeleteRelation');
        comp.selectedRelationId.set(10);
        keyDown('Delete');
        expect(spy).toHaveBeenCalledWith({
            relationId: 10,
            idProject: 10,
            idIssuePublic: 1
        });
    });

    it('Backspace on a selected outbound relation calls onDeleteRelation', () => {
        const spy = vi.spyOn(comp, 'onDeleteRelation');
        comp.selectedRelationId.set(10);
        keyDown('Backspace');
        expect(spy).toHaveBeenCalled();
    });

    it('Delete on an inbound relation does nothing', () => {
        const spy = vi.spyOn(comp, 'onDeleteRelation');
        comp.selectedRelationId.set(99);
        keyDown('Delete');
        expect(spy).not.toHaveBeenCalled();
    });

    it('Delete with no idProject (empty tasks) does nothing', async () => {
        TestBed.resetTestingModule();
        localStorage.clear();
        const { fixture, comp } = await createGanttFixture({
            tasks: [],
            relations: [
                {
                    idIssueRelation: 10,
                    direction: 'outbound',
                    relationType: IssueRelationType.Schedule,
                    from: { idIssuePublic: 1 },
                    to: { idIssuePublic: 2 }
                } as any
            ]
        });
        const spy = vi.spyOn(comp, 'onDeleteRelation');
        comp.selectedRelationId.set(10);
        fixture.nativeElement.dispatchEvent(
            new KeyboardEvent('keydown', { key: 'Delete', bubbles: true })
        );
        fixture.detectChanges();
        expect(spy).not.toHaveBeenCalled();
    });
});

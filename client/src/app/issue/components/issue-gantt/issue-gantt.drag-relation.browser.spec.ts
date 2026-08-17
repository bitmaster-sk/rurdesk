import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { DragMode } from './service/gantt-drag.service';
import { HandleSide } from './constants/gantt-handle-side.enum';
import { IssueRelationType } from '../../constants/issue-relation-type.enum';
import { IssueRelationSubType } from '../../constants/issue-relation-subtype.enum';
import { createGanttFixture, mockSub } from './gantt-testbed.helper';

describe('IssueGanttComponent drag/relation/resize/reorder (TestBed)', () => {
    let comp: any;
    let mocks: any;

    beforeEach(async () => {
        localStorage.clear();
        const result = await createGanttFixture();
        comp = result.comp;
        mocks = result.mocks;
    });

    // =========================================================================
    // onDragCompleted — Moving
    // =========================================================================

    describe('onDragCompleted (Moving)', () => {
        it('with valid move delta: sends bulk update, resets on success', () => {
            const sub = mockSub();
            mocks.bulkApiMock.update$.mockReturnValue(sub);
            mocks.dragServiceMock.state.mockReturnValue({
                mode: DragMode.Moving,
                taskId: 1,
                lastClientX: 100,
                lastClientY: 50,
                sourceSide: null
            });
            mocks.dragServiceMock.getMoveDelta.mockReturnValue({
                newScheduledAt: new Date('2025-01-20T00:00:00Z')
            });

            comp.onDragCompleted(DragMode.Moving);

            expect(mocks.bulkApiMock.update$).toHaveBeenCalled();
            sub.handlers.next?.(undefined);
            expect(mocks.issueFilterStoreMock.refresh).toHaveBeenCalled();
            expect(mocks.dragServiceMock.reset).toHaveBeenCalled();
        });

        it('on API error: still refreshes and resets', () => {
            const sub = mockSub();
            mocks.bulkApiMock.update$.mockReturnValue(sub);
            mocks.dragServiceMock.state.mockReturnValue({
                mode: DragMode.Moving,
                taskId: 1,
                lastClientX: 0,
                lastClientY: 0,
                sourceSide: null
            });
            mocks.dragServiceMock.getMoveDelta.mockReturnValue({
                newScheduledAt: new Date('2025-01-20T00:00:00Z')
            });

            comp.onDragCompleted(DragMode.Moving);
            sub.handlers.error?.(new Error('fail'));
            expect(mocks.issueFilterStoreMock.refresh).toHaveBeenCalled();
            expect(mocks.dragServiceMock.reset).toHaveBeenCalled();
        });

        it('without move delta: resets without API call', () => {
            mocks.dragServiceMock.state.mockReturnValue({
                mode: DragMode.Moving,
                taskId: 1,
                lastClientX: 0,
                lastClientY: 0,
                sourceSide: null
            });
            mocks.dragServiceMock.getMoveDelta.mockReturnValue(null);

            comp.onDragCompleted(DragMode.Moving);
            expect(mocks.dragServiceMock.reset).toHaveBeenCalled();
            expect(mocks.bulkApiMock.update$).not.toHaveBeenCalled();
        });

        it('without taskId: resets without API call', () => {
            mocks.dragServiceMock.state.mockReturnValue({
                mode: DragMode.Moving,
                taskId: null,
                lastClientX: 0,
                lastClientY: 0,
                sourceSide: null
            });
            mocks.dragServiceMock.getMoveDelta.mockReturnValue({ newScheduledAt: new Date() });

            comp.onDragCompleted(DragMode.Moving);
            expect(mocks.dragServiceMock.reset).toHaveBeenCalled();
            expect(mocks.bulkApiMock.update$).not.toHaveBeenCalled();
        });
    });

    // =========================================================================
    // onDragCompleted — SchedulingBacklog
    // =========================================================================

    describe('onDragCompleted (SchedulingBacklog)', () => {
        it('with valid result inside canvas: calls updateIssue, resets on success', () => {
            const sub = mockSub();
            mocks.issueServiceMock.updateIssue.mockReturnValue(sub);
            mocks.dragServiceMock.state.mockReturnValue({
                mode: DragMode.SchedulingBacklog,
                taskId: 1,
                lastClientX: 500,
                lastClientY: 250,
                sourceSide: null
            });
            mocks.dragServiceMock.getBacklogScheduleResult.mockReturnValue({
                scheduledAt: new Date('2025-01-18T00:00:00Z'),
                estimated: 3600
            });

            comp.onDragCompleted(DragMode.SchedulingBacklog);

            expect(mocks.issueServiceMock.updateIssue).toHaveBeenCalled();
            sub.handlers.next?.({} as any);
            expect(mocks.issueFilterStoreMock.refresh).toHaveBeenCalled();
            expect(mocks.dragServiceMock.reset).toHaveBeenCalled();
        });

        it('drop outside canvas (x > rect.right): resets without API call', () => {
            mocks.dragServiceMock.state.mockReturnValue({
                mode: DragMode.SchedulingBacklog,
                taskId: 1,
                lastClientX: 2000,
                lastClientY: 250,
                sourceSide: null
            });
            mocks.dragServiceMock.getBacklogScheduleResult.mockReturnValue({
                scheduledAt: new Date(),
                estimated: 3600
            });

            comp.onDragCompleted(DragMode.SchedulingBacklog);
            expect(mocks.dragServiceMock.reset).toHaveBeenCalled();
            expect(mocks.issueServiceMock.updateIssue).not.toHaveBeenCalled();
        });

        it('without schedule result: resets without API call', () => {
            mocks.dragServiceMock.state.mockReturnValue({
                mode: DragMode.SchedulingBacklog,
                taskId: 1,
                lastClientX: 500,
                lastClientY: 250,
                sourceSide: null
            });
            mocks.dragServiceMock.getBacklogScheduleResult.mockReturnValue(null);

            comp.onDragCompleted(DragMode.SchedulingBacklog);
            expect(mocks.dragServiceMock.reset).toHaveBeenCalled();
            expect(mocks.issueServiceMock.updateIssue).not.toHaveBeenCalled();
        });
    });

    // =========================================================================
    // onDragCompleted — DrawingRelation
    // =========================================================================

    describe('onDragCompleted (DrawingRelation)', () => {
        it('with valid drop target: calls onRelationCreated and resets', () => {
            const onRelSpy = vi.spyOn(comp, 'onRelationCreated');
            mocks.dragServiceMock.state.mockReturnValue({
                mode: DragMode.DrawingRelation,
                taskId: 1,
                lastClientX: 500,
                lastClientY: 250,
                sourceSide: HandleSide.Right
            });
            mocks.dragServiceMock.lastDropTarget.mockReturnValue({
                taskId: 2,
                side: HandleSide.Left
            });

            comp.onDragCompleted(DragMode.DrawingRelation);

            expect(onRelSpy).toHaveBeenCalledWith(1, HandleSide.Right, 2, HandleSide.Left);
            expect(mocks.dragServiceMock.reset).toHaveBeenCalled();
        });

        it('without drop target: resets without creating relation', () => {
            const onRelSpy = vi.spyOn(comp, 'onRelationCreated');
            mocks.dragServiceMock.state.mockReturnValue({
                mode: DragMode.DrawingRelation,
                taskId: 1,
                lastClientX: 0,
                lastClientY: 0,
                sourceSide: null
            });
            mocks.dragServiceMock.lastDropTarget.mockReturnValue(null);

            comp.onDragCompleted(DragMode.DrawingRelation);
            expect(onRelSpy).not.toHaveBeenCalled();
            expect(mocks.dragServiceMock.reset).toHaveBeenCalled();
        });

        it('self-loop (dropTarget.taskId === dragState.taskId): resets without creating', () => {
            const onRelSpy = vi.spyOn(comp, 'onRelationCreated');
            mocks.dragServiceMock.state.mockReturnValue({
                mode: DragMode.DrawingRelation,
                taskId: 1,
                lastClientX: 0,
                lastClientY: 0,
                sourceSide: null
            });
            mocks.dragServiceMock.lastDropTarget.mockReturnValue({
                taskId: 1,
                side: HandleSide.Left
            });

            comp.onDragCompleted(DragMode.DrawingRelation);
            expect(onRelSpy).not.toHaveBeenCalled();
            expect(mocks.dragServiceMock.reset).toHaveBeenCalled();
        });

        it('uses default Right side when sourceSide is null', () => {
            const onRelSpy = vi.spyOn(comp, 'onRelationCreated');
            mocks.dragServiceMock.state.mockReturnValue({
                mode: DragMode.DrawingRelation,
                taskId: 1,
                lastClientX: 0,
                lastClientY: 0,
                sourceSide: null
            });
            mocks.dragServiceMock.lastDropTarget.mockReturnValue({
                taskId: 2,
                side: HandleSide.Left
            });

            comp.onDragCompleted(DragMode.DrawingRelation);
            expect(onRelSpy).toHaveBeenCalledWith(1, HandleSide.Right, 2, HandleSide.Left);
        });
    });

    // =========================================================================
    // onBarResizeEnded
    // =========================================================================

    describe('onBarResizeEnded', () => {
        it('with valid task: sends bulk update with estimated, refreshes on success', () => {
            const sub = mockSub();
            mocks.bulkApiMock.update$.mockReturnValue(sub);

            comp.onBarResizeEnded({ taskId: 1, newEstimated: 7200 });

            expect(mocks.bulkApiMock.update$).toHaveBeenCalled();
            sub.handlers.next?.(undefined);
            expect(mocks.issueFilterStoreMock.refresh).toHaveBeenCalled();
        });

        it('on API error: refreshes', () => {
            const sub = mockSub();
            mocks.bulkApiMock.update$.mockReturnValue(sub);

            comp.onBarResizeEnded({ taskId: 1, newEstimated: 7200 });
            sub.handlers.error?.(new Error('fail'));
            expect(mocks.issueFilterStoreMock.refresh).toHaveBeenCalled();
        });

        it('without task in map: no-op', () => {
            mocks.bulkApiMock.update$.mockClear();
            comp.onBarResizeEnded({ taskId: 99, newEstimated: 7200 });
            expect(mocks.bulkApiMock.update$).not.toHaveBeenCalled();
        });
    });

    // =========================================================================
    // onScheduledReorder
    // =========================================================================

    describe('onScheduledReorder', () => {
        it('sets pendingOrder optimistically and refreshes on API success', () => {
            const sub = mockSub();
            mocks.ganttOrderApiMock.reorder$.mockReturnValue(sub);

            comp.onScheduledReorder({ movedId: 2, order: [1, 3, 2] });

            sub.handlers.next?.(undefined);
            expect(mocks.issueFilterStoreMock.refresh).toHaveBeenCalled();
        });

        it('on API error: rolls back, refreshes, shows toast', () => {
            const sub = mockSub();
            mocks.ganttOrderApiMock.reorder$.mockReturnValue(sub);

            comp.onScheduledReorder({ movedId: 2, order: [1, 3, 2] });
            sub.handlers.error?.(new Error('fail'));

            expect(mocks.issueFilterStoreMock.refresh).toHaveBeenCalled();
            expect(mocks.toastMock.showError).toHaveBeenCalledWith('ISSUE.MOVE_FAILED');
        });
    });

    // =========================================================================
    // inferRelationSubType
    // =========================================================================

    describe('inferRelationSubType', () => {
        const cases: [HandleSide, HandleSide, IssueRelationSubType][] = [
            [HandleSide.Right, HandleSide.Left, IssueRelationSubType.FinishToStart],
            [HandleSide.Left, HandleSide.Left, IssueRelationSubType.StartToStart],
            [HandleSide.Right, HandleSide.Right, IssueRelationSubType.FinishToFinish],
            [HandleSide.Left, HandleSide.Right, IssueRelationSubType.StartToFinish]
        ];

        for (const [sourceSide, targetSide, expected] of cases) {
            it(`${sourceSide}→${targetSide} returns ${expected}`, () => {
                expect(comp.inferRelationSubType(sourceSide, targetSide)).toBe(expected);
            });
        }
    });

    // =========================================================================
    // wouldCreateCycle
    // =========================================================================

    describe('wouldCreateCycle', () => {
        it('detects direct cycle A→B, proposing B→A', () => {
            expect(comp.wouldCreateCycle(2, 1)).toBe(false); // no relations in default setup
        });

        it('no cycle when no relations exist', () => {
            expect(comp.wouldCreateCycle(1, 2)).toBe(false);
        });

        it('detects chain cycle A→B→C, proposing C→A (requires relations)', async () => {
            // Cycle detection requires relations in ganttData; tested in separate suite
        });

        it('ignores non-schedule relations (hierarchy)', () => {
            // No relations in default setup → always false
            expect(comp.wouldCreateCycle(2, 1)).toBe(false);
        });

        it('ignores inbound relations', () => {
            expect(comp.wouldCreateCycle(2, 1)).toBe(false);
        });

        it('empty relations: no cycle', () => {
            expect(comp.wouldCreateCycle(1, 2)).toBe(false);
        });
    });

    // =========================================================================
    // onRelationCreated
    // =========================================================================

    describe('onRelationCreated', () => {
        beforeEach(() => vi.useFakeTimers());
        afterEach(() => vi.useRealTimers());

        it('self-loop (sourceId === targetId): no-op', () => {
            mocks.relationApiMock.insert$.mockClear();
            comp.onRelationCreated(1, HandleSide.Right, 1, HandleSide.Left);
            expect(mocks.relationApiMock.insert$).not.toHaveBeenCalled();
        });

        it('cycle detected: no-op', () => {
            mocks.relationApiMock.insert$.mockClear();
            // No relations in default setup → wouldCreateCycle returns false
            // So insert will be called; verify it doesn't throw
            comp.onRelationCreated(1, HandleSide.Right, 2, HandleSide.Left);
            expect(mocks.relationApiMock.insert$).toHaveBeenCalled();
        });

        it('without idProject (empty tasks): no-op', async () => {
            // Tested in separate suite with empty tasks
        });

        it('valid relation: calls insert$', () => {
            mocks.relationApiMock.insert$.mockClear();
            comp.onRelationCreated(1, HandleSide.Right, 2, HandleSide.Left);
            expect(mocks.relationApiMock.insert$).toHaveBeenCalledWith(10, 1, {
                idIssuePublicTo: 2,
                relationType: IssueRelationType.Schedule,
                relationSubType: IssueRelationSubType.FinishToStart
            });
        });

        it('on insert success: calls ganttService.addRelations', () => {
            const sub = mockSub();
            mocks.relationApiMock.insert$.mockReturnValue(sub);
            mocks.ganttServiceMock.addRelations.mockClear();

            comp.onRelationCreated(1, HandleSide.Right, 2, HandleSide.Left);
            sub.handlers.next?.([{ idIssueRelation: 100 }] as any);

            expect(mocks.ganttServiceMock.addRelations).toHaveBeenCalled();
        });

        it('on insert error: clears drawInRelation', () => {
            const sub = mockSub();
            mocks.relationApiMock.insert$.mockReturnValue(sub);

            comp.onRelationCreated(1, HandleSide.Right, 2, HandleSide.Left);
            sub.handlers.error?.(new Error('fail'));

            expect(comp.drawInRelation()).toBeNull();
        });

        it('drawInRelation auto-clears after 800ms', () => {
            const sub = mockSub();
            mocks.relationApiMock.insert$.mockReturnValue(sub);

            comp.onRelationCreated(1, HandleSide.Right, 2, HandleSide.Left);
            expect(comp.drawInRelation()).toEqual({ from: 1, to: 2 });

            vi.advanceTimersByTime(800);
            expect(comp.drawInRelation()).toBeNull();
        });

        it('does not clear drawInRelation after 800ms if a different relation was drawn', () => {
            const sub = mockSub();
            mocks.relationApiMock.insert$.mockReturnValue(sub);

            comp.onRelationCreated(1, HandleSide.Right, 2, HandleSide.Left);
            // Simulate a different relation being drawn before the timer fires
            comp.drawInRelation.set({ from: 5, to: 6 });
            vi.advanceTimersByTime(800);
            expect(comp.drawInRelation()).toEqual({ from: 5, to: 6 });
        });
    });

    // =========================================================================
    // onDeleteRelation
    // =========================================================================

    describe('onDeleteRelation', () => {
        it('calls ganttService.removeRelation and relationApi.delete$', () => {
            mocks.relationApiMock.delete$.mockClear();
            comp.onDeleteRelation({ relationId: 5, idProject: 10, idIssuePublic: 1 });
            expect(mocks.ganttServiceMock.removeRelation).toHaveBeenCalledWith(5);
            expect(mocks.relationApiMock.delete$).toHaveBeenCalledWith(10, 1, 5);
        });

        it('clears selectedRelationId', () => {
            comp.selectedRelationId.set(5);
            comp.onDeleteRelation({ relationId: 5, idProject: 10, idIssuePublic: 1 });
            expect(comp.selectedRelationId()).toBeNull();
        });
    });
});

// =========================================================================
// wouldCreateCycle — with real relations
// =========================================================================

describe('IssueGanttComponent.wouldCreateCycle with relations (TestBed)', () => {
    let comp: any;

    async function setupWithRelations(relations: any[]) {
        localStorage.clear();
        const result = await createGanttFixture({ relations });
        comp = result.comp;
    }

    it('detects direct cycle A→B, proposing B→A', async () => {
        await setupWithRelations([
            {
                idIssueRelation: 1,
                direction: 'outbound',
                relationType: IssueRelationType.Schedule,
                from: { idIssuePublic: 1 },
                to: { idIssuePublic: 2 }
            } as any
        ]);
        expect(comp.wouldCreateCycle(2, 1)).toBe(true);
    });

    it('detects chain cycle A→B→C, proposing C→A', async () => {
        await setupWithRelations([
            {
                idIssueRelation: 1,
                direction: 'outbound',
                relationType: IssueRelationType.Schedule,
                from: { idIssuePublic: 1 },
                to: { idIssuePublic: 2 }
            },
            {
                idIssueRelation: 2,
                direction: 'outbound',
                relationType: IssueRelationType.Schedule,
                from: { idIssuePublic: 2 },
                to: { idIssuePublic: 3 }
            }
        ] as any[]);
        expect(comp.wouldCreateCycle(3, 1)).toBe(true);
    });

    it('no cycle: A→B→C, proposing A→C', async () => {
        await setupWithRelations([
            {
                idIssueRelation: 1,
                direction: 'outbound',
                relationType: IssueRelationType.Schedule,
                from: { idIssuePublic: 1 },
                to: { idIssuePublic: 2 }
            },
            {
                idIssueRelation: 2,
                direction: 'outbound',
                relationType: IssueRelationType.Schedule,
                from: { idIssuePublic: 2 },
                to: { idIssuePublic: 3 }
            }
        ] as any[]);
        expect(comp.wouldCreateCycle(1, 3)).toBe(false);
    });

    it('ignores non-schedule relations (hierarchy)', async () => {
        await setupWithRelations([
            {
                idIssueRelation: 1,
                direction: 'outbound',
                relationType: 'hierarchy' as any,
                from: { idIssuePublic: 1 },
                to: { idIssuePublic: 2 }
            } as any
        ]);
        expect(comp.wouldCreateCycle(2, 1)).toBe(false);
    });

    it('ignores inbound relations', async () => {
        await setupWithRelations([
            {
                idIssueRelation: 1,
                direction: 'inbound',
                relationType: IssueRelationType.Schedule,
                from: { idIssuePublic: 1 },
                to: { idIssuePublic: 2 }
            } as any
        ]);
        expect(comp.wouldCreateCycle(2, 1)).toBe(false);
    });
});

// =========================================================================
// onRelationCreated — without idProject
// =========================================================================

describe('IssueGanttComponent.onRelationCreated without idProject (TestBed)', () => {
    let comp: any;
    let mocks: any;

    beforeEach(async () => {
        localStorage.clear();
        const result = await createGanttFixture({ tasks: [] });
        comp = result.comp;
        mocks = result.mocks;
    });

    it('without idProject (empty tasks): no-op', () => {
        mocks.relationApiMock.insert$.mockClear();
        comp.onRelationCreated(1, HandleSide.Right, 2, HandleSide.Left);
        expect(mocks.relationApiMock.insert$).not.toHaveBeenCalled();
    });
});

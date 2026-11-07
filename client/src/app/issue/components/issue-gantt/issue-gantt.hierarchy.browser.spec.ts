import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';
import { IssueGanttComponent } from './issue-gantt.component';
import { DragMode } from './service/gantt-drag.service';
import { IssueRelationType } from '../../constants/issue-relation-type.enum';
import { IssueRelationDirection } from '../../constants/issue-relation-direction.enum';
import { IssueRelationSubType } from '../../constants/issue-relation-subtype.enum';
import { IssueRelationRef, ReadIssueRelationDto } from '../../model/issue-relation.model';
import { createGanttFixture, makeTask, mockSub } from './gantt-testbed.helper';
import { STORAGE_KEY_WBS_COLLAPSED } from './constants/gantt-storage-keys';
import { GanttSummaryChoice } from './constants/gantt-summary-choice.enum';
import { GanttCascadeService } from './service/gantt-cascade.service';

const ref = (idIssuePublic: number): IssueRelationRef => ({
    idIssuePublic,
    title: '',
    idSeverity: null,
    idState: null,
    assignedTo: null,
    updateAt: '',
    qualityScore: null
});

const hierarchy = (idParent: number, idChild: number): ReadIssueRelationDto => ({
    idIssueRelation: idParent * 100 + idChild,
    relationType: IssueRelationType.Hierarchy,
    relationSubType: null,
    lagMinutes: null,
    direction: IssueRelationDirection.Outbound,
    label: 'child',
    inverseLabel: 'parent',
    from: ref(idParent),
    to: ref(idChild),
    createdAt: '',
    createdBy: 0
});

const schedule = (idFrom: number, idTo: number): ReadIssueRelationDto => ({
    ...hierarchy(idFrom, idTo),
    idIssueRelation: 5000 + idFrom * 100 + idTo,
    relationType: IssueRelationType.Schedule,
    relationSubType: IssueRelationSubType.FinishToStart,
    lagMinutes: 0,
    label: 'blocks',
    inverseLabel: 'blocked by'
});

const useRealCascade = (mocks: any): void => {
    const real = new GanttCascadeService();
    mocks.cascadeServiceMock.computeTreeCascade.mockImplementation(
        real.computeTreeCascade.bind(real)
    );
    mocks.cascadeServiceMock.summaryMoveFloor.mockImplementation(real.summaryMoveFloor.bind(real));
};

const dragTo = (mocks: any, taskId: number, newScheduledAt: Date): void => {
    mocks.bulkApiMock.update$.mockReturnValue(mockSub());
    mocks.dragServiceMock.state.mockReturnValue({
        mode: DragMode.Moving,
        taskId,
        lastClientX: 0,
        lastClientY: 0,
        sourceSide: null
    });
    mocks.dragServiceMock.getMoveDelta.mockReturnValue({ newScheduledAt });
};

const day = (n: number): Date => new Date(Date.UTC(2025, 0, 13 + n));

const tasks = [
    makeTask({
        idIssuePublic: 1,
        title: 'Vývoj',
        scheduledAt: day(0),
        estimated: 3 * 86400,
        isSummary: true
    }),
    makeTask({ idIssuePublic: 2, title: 'Backend', scheduledAt: day(0), estimated: 86400 }),
    makeTask({ idIssuePublic: 3, title: 'Frontend', scheduledAt: day(2), estimated: 86400 }),
    makeTask({ idIssuePublic: 4, title: 'Testovanie', scheduledAt: day(4), estimated: 86400 })
];
const relations = [hierarchy(1, 2), hierarchy(1, 3)];

const ids = (comp: any): number[] => comp.visibleTasks().map((t: any) => t.idIssuePublic);

const recreate = async (): Promise<any> => {
    const fixture = TestBed.createComponent(IssueGanttComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return fixture.componentInstance;
};

describe('IssueGanttComponent hierarchy (TestBed)', () => {
    let comp: any;
    let mocks: any;

    beforeEach(async () => {
        localStorage.clear();
        const result = await createGanttFixture({ tasks, hierarchyRelations: relations });
        comp = result.comp;
        mocks = result.mocks;
    });

    it('lays the tasks out as a numbered tree', () => {
        expect(comp.treeRows().map((r: any) => [r.task.idIssuePublic, r.wbsCode, r.depth])).toEqual(
            [
                [1, '1', 0],
                [2, '1.1', 1],
                [3, '1.2', 1],
                [4, '2', 0]
            ]
        );
    });

    it('hides the subtree of a collapsed summary and remembers it per project', async () => {
        comp.onToggleCollapse(1);
        expect(ids(comp)).toEqual([1, 4]);
        expect(JSON.parse(localStorage.getItem(STORAGE_KEY_WBS_COLLAPSED)!)).toEqual({ 10: [1] });

        expect(ids(await recreate())).toEqual([1, 4]);
    });

    it('ignores a broken collapse store', async () => {
        localStorage.setItem(STORAGE_KEY_WBS_COLLAPSED, '{not json');
        expect(ids(await recreate())).toEqual([1, 2, 3, 4]);
    });

    it('collapses to an outline level and expands everything again', () => {
        comp.onOutlineLevelChange(1);
        expect(ids(comp)).toEqual([1, 4]);
        expect(comp.activeOutlineLevel()).toBe(1);
        comp.onOutlineLevelChange(0);
        expect(ids(comp)).toEqual([1, 2, 3, 4]);
        expect(comp.activeOutlineLevel()).toBe(0);
    });

    it('highlights no outline level once a row is folded by hand', () => {
        comp.onToggleCollapse(1);
        expect(comp.activeOutlineLevel()).toBeNull();
        comp.onToggleCollapse(1);
        expect(comp.activeOutlineLevel()).toBe(0);
    });

    it('selects a clicked row and indents it with Tab', () => {
        mocks.relationApiMock.setParent$.mockReturnValue(mockSub());
        comp.onWbsTaskClicked({ taskId: 4, isBacklog: false });
        expect(comp.selectedTaskId()).toBe(4);
        comp.onKeyDown(new KeyboardEvent('keydown', { key: 'Tab' }));
        expect(mocks.relationApiMock.setParent$).toHaveBeenCalledWith(10, 4, { idParent: 1 });
    });

    it('moves every scheduled leaf of a summary by the same amount', () => {
        useRealCascade(mocks);
        const sub = mockSub();
        mocks.bulkApiMock.update$.mockReturnValue(sub);
        mocks.dragServiceMock.state.mockReturnValue({
            mode: DragMode.Moving,
            taskId: 1,
            lastClientX: 0,
            lastClientY: 0,
            sourceSide: null
        });
        mocks.dragServiceMock.getMoveDelta.mockReturnValue({ newScheduledAt: day(1) });

        comp.onDragCompleted(DragMode.Moving);

        const [, entries] = mocks.bulkApiMock.update$.mock.calls[0];
        expect(entries).toEqual([
            { idIssuePublic: 2, scheduledAt: day(1).toISOString() },
            { idIssuePublic: 3, scheduledAt: day(3).toISOString() }
        ]);
    });

    it('also moves the subtasks of a summary that the gantt has not loaded', async () => {
        TestBed.resetTestingModule();
        const loaded = await createGanttFixture({
            tasks,
            hierarchyRelations: [...relations, hierarchy(1, 9)]
        });
        useRealCascade(loaded.mocks);
        loaded.mocks.issueApiMock.load$.mockReturnValue(
            of([makeTask({ idIssuePublic: 9, scheduledAt: day(5), estimated: 86400 })])
        );
        loaded.mocks.bulkApiMock.update$.mockReturnValue(mockSub());
        loaded.mocks.dragServiceMock.state.mockReturnValue({
            mode: DragMode.Moving,
            taskId: 1,
            lastClientX: 0,
            lastClientY: 0,
            sourceSide: null
        });
        loaded.mocks.dragServiceMock.getMoveDelta.mockReturnValue({ newScheduledAt: day(1) });

        loaded.comp.onDragCompleted(DragMode.Moving);

        expect(loaded.mocks.issueApiMock.load$.mock.calls[0][0].idsIssuePublic).toEqual([9]);
        const [, entries] = loaded.mocks.bulkApiMock.update$.mock.calls[0];
        expect(entries).toContainEqual({ idIssuePublic: 9, scheduledAt: day(6).toISOString() });
        expect(entries).toHaveLength(3);
    });

    it('also moves the hidden subtasks of a summary that the cascade pushes', async () => {
        TestBed.resetTestingModule();
        const loaded = await createGanttFixture({
            tasks,
            relations: [schedule(4, 1)],
            hierarchyRelations: [...relations, hierarchy(1, 9)]
        });
        useRealCascade(loaded.mocks);
        loaded.mocks.issueApiMock.load$.mockReturnValue(
            of([makeTask({ idIssuePublic: 9, scheduledAt: day(1), estimated: 86400 })])
        );
        dragTo(loaded.mocks, 4, day(3));

        loaded.comp.onDragCompleted(DragMode.Moving);

        expect(loaded.mocks.issueApiMock.load$.mock.calls[0][0].idsIssuePublic).toEqual([9]);
        const [, entries] = loaded.mocks.bulkApiMock.update$.mock.calls[0];
        expect(entries).toEqual(
            expect.arrayContaining([
                { idIssuePublic: 4, scheduledAt: day(3).toISOString() },
                { idIssuePublic: 2, scheduledAt: day(4).toISOString() },
                { idIssuePublic: 3, scheduledAt: day(6).toISOString() },
                { idIssuePublic: 9, scheduledAt: day(5).toISOString() }
            ])
        );
        expect(entries).toHaveLength(4);
    });

    it('pushes the dependents of a summary whose subtasks are all hidden', async () => {
        TestBed.resetTestingModule();
        const loaded = await createGanttFixture({
            tasks: [
                makeTask({
                    idIssuePublic: 1,
                    title: 'Vývoj',
                    scheduledAt: day(0),
                    estimated: 2 * 86400,
                    isSummary: true
                }),
                makeTask({
                    idIssuePublic: 4,
                    title: 'Testovanie',
                    scheduledAt: day(2),
                    estimated: 86400
                })
            ],
            relations: [schedule(1, 4)],
            hierarchyRelations: [hierarchy(1, 9)]
        });
        useRealCascade(loaded.mocks);
        loaded.mocks.issueApiMock.load$.mockReturnValue(
            of([makeTask({ idIssuePublic: 9, scheduledAt: day(0), estimated: 2 * 86400 })])
        );
        dragTo(loaded.mocks, 1, day(1));

        loaded.comp.onDragCompleted(DragMode.Moving);

        const [, entries] = loaded.mocks.bulkApiMock.update$.mock.calls[0];
        expect(entries).toEqual(
            expect.arrayContaining([
                { idIssuePublic: 9, scheduledAt: day(1).toISOString() },
                { idIssuePublic: 4, scheduledAt: day(3).toISOString() }
            ])
        );
        expect(entries).toHaveLength(2);
    });

    it('saves nothing and reports it when the hidden subtasks cannot be loaded', async () => {
        TestBed.resetTestingModule();
        const loaded = await createGanttFixture({
            tasks,
            hierarchyRelations: [...relations, hierarchy(1, 9)]
        });
        loaded.mocks.issueApiMock.load$.mockReturnValue(throwError(() => new Error('down')));
        dragTo(loaded.mocks, 1, day(1));

        loaded.comp.onDragCompleted(DragMode.Moving);

        expect(loaded.mocks.bulkApiMock.update$).not.toHaveBeenCalled();
        expect(loaded.mocks.toastMock.showError).toHaveBeenCalledWith('ISSUE.UPDATE_FAILED');
        expect(loaded.mocks.issueFilterStoreMock.refresh).toHaveBeenCalled();
    });

    it('stops the ghost of a dragged summary where its predecessor ends', async () => {
        TestBed.resetTestingModule();
        const loaded = await createGanttFixture({
            tasks: [
                makeTask({
                    idIssuePublic: 6,
                    title: 'Analýza',
                    scheduledAt: day(-2),
                    estimated: 86400
                }),
                ...tasks
            ],
            relations: [schedule(6, 1)],
            hierarchyRelations: relations
        });
        useRealCascade(loaded.mocks);
        loaded.mocks.timelineServiceMock.toPixel.mockImplementation((date: Date) => date.getTime());
        loaded.mocks.dragServiceMock.state.mockReturnValue({
            mode: DragMode.Moving,
            taskId: 1,
            currentDate: day(-2),
            lastClientX: 0,
            lastClientY: 0,
            sourceSide: null
        });
        const comp = await recreate();

        const bar = comp.ghostBars().find((b: any) => b.taskId === 1);
        expect(bar.left).toBe(day(-1).getTime());
        expect(bar.tooltipText).toBe(day(-1).toISOString());
    });

    it('sends every row in the first reorder, also the subtasks of a context parent', async () => {
        TestBed.resetTestingModule();
        const context = await createGanttFixture({
            tasks: tasks.slice(1),
            contextTasks: [tasks[0]],
            hierarchyRelations: relations
        });
        context.mocks.ganttOrderApiMock.reorder$.mockReturnValue(mockSub());

        context.comp.onTreeDropped({ movedId: 3, targetId: 2, kind: 'before' });

        const [, body] = context.mocks.ganttOrderApiMock.reorder$.mock.calls[0];
        expect(body).toEqual({ movedId: 3, order: [3, 2, 4] });
    });

    it('keeps the selected task selected when the rows above it change', () => {
        comp.onWbsTaskClicked({ taskId: 4, isBacklog: false });
        comp.onToggleCollapse(1);
        expect(comp.selectedTaskId()).toBe(4);
        expect(comp.selectedTaskIndex()).toBe(1);
        comp.onToggleCollapse(1);
        expect(comp.selectedTaskIndex()).toBe(3);
    });

    it('drops the selection when the selected task gets folded away', () => {
        comp.onWbsTaskClicked({ taskId: 3, isBacklog: false });
        comp.onToggleCollapse(1);
        expect(comp.selectedTaskId()).toBeNull();
    });

    it('opens the quick actions of a summary without rescheduling', () => {
        const event = new MouseEvent('contextmenu', { clientX: 0, clientY: 0 });
        comp.onBarContextMenu({ taskId: 1, event });
        comp.onBarContextMenu({ taskId: 4, event });
        const { show } = comp.quickActionsRef();
        expect(show.mock.calls[0][2]).toEqual({ showReschedule: false });
        expect(show.mock.calls[1][2]).toEqual({ showReschedule: true });
    });

    it('indents with Alt+Shift+Right under the previous sibling', () => {
        mocks.relationApiMock.setParent$.mockReturnValue(mockSub());
        comp.selectedTaskIndex.set(3);
        comp.onKeyDown(
            new KeyboardEvent('keydown', { key: 'ArrowRight', altKey: true, shiftKey: true })
        );
        expect(mocks.relationApiMock.setParent$).toHaveBeenCalledWith(10, 4, { idParent: 1 });
    });

    it('outdents with Alt+Shift+Left and places the task after its old parent', () => {
        const sub = mockSub<{
            relations: ReadIssueRelationDto[];
            idRelationRemoved: number | null;
        }>();
        mocks.relationApiMock.setParent$.mockReturnValue(sub);
        mocks.ganttOrderApiMock.reorder$.mockReturnValue(mockSub());
        comp.selectedTaskIndex.set(2);

        comp.onKeyDown(
            new KeyboardEvent('keydown', { key: 'ArrowLeft', altKey: true, shiftKey: true })
        );
        expect(mocks.relationApiMock.setParent$).toHaveBeenCalledWith(10, 3, { idParent: null });

        sub.handlers.next?.({ relations: [], idRelationRemoved: 103 });
        expect(mocks.ganttServiceMock.removeRelation).toHaveBeenCalledWith(103);
        const [, body] = mocks.ganttOrderApiMock.reorder$.mock.calls[0];
        expect(body).toEqual({ movedId: 3, order: [1, 2, 3, 4] });
    });

    it('asks before a task with its own estimate becomes a summary', () => {
        comp.onTreeDropped({ movedId: 2, targetId: 4, kind: 'into' });

        expect(mocks.relationApiMock.setParent$).not.toHaveBeenCalled();
        expect(comp.pendingParent()).toMatchObject({ id: 2, idParent: 4, estimated: 86400 });
    });

    it('asks the server to move the own estimate to a subtask in the same change', () => {
        mocks.relationApiMock.setParent$.mockReturnValue(mockSub());
        comp.onTreeDropped({ movedId: 2, targetId: 4, kind: 'into' });

        comp.onSummaryChosen(GanttSummaryChoice.MoveToSubtask);

        expect(comp.pendingParent()).toBeNull();
        expect(mocks.relationApiMock.setParent$).toHaveBeenCalledWith(10, 2, {
            idParent: 4,
            remainingWork: {
                title: expect.stringContaining('ISSUE.GANTT.REMAINING_WORK'),
                description: 'ISSUE.GANTT.REMAINING_WORK'
            }
        });
        expect(mocks.issueApiMock.insert$).not.toHaveBeenCalled();
    });

    it('drops the own estimate without creating a subtask', () => {
        const sub = mockSub<{
            relations: ReadIssueRelationDto[];
            idRelationRemoved: number | null;
        }>();
        mocks.relationApiMock.setParent$.mockReturnValue(sub);
        comp.onTreeDropped({ movedId: 2, targetId: 4, kind: 'into' });

        comp.onSummaryChosen(GanttSummaryChoice.Discard);
        sub.handlers.next?.({ relations: [], idRelationRemoved: 102 });

        expect(mocks.relationApiMock.setParent$).toHaveBeenCalledWith(10, 2, { idParent: 4 });
        expect(mocks.issueApiMock.insert$).not.toHaveBeenCalled();
    });

    it('keeps the tree as it was when the question is cancelled', () => {
        comp.onTreeDropped({ movedId: 2, targetId: 4, kind: 'into' });

        comp.onSummaryCancelled();

        expect(comp.pendingParent()).toBeNull();
        expect(mocks.relationApiMock.setParent$).not.toHaveBeenCalled();
    });
});

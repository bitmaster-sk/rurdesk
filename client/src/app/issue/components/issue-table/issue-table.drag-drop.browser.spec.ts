import { describe, it, expect, vi, beforeEach } from 'vitest';
import { createTableFixture, makeIssue } from './table-testbed.helper';
import { IssueRelationType } from '../../constants/issue-relation-type.enum';
import { IssueRelationSubType } from '../../constants/issue-relation-subtype.enum';
import { RelationDropEvent } from './components/issue-table-drop-zone/issue-table-drop-zone.component';

function makeDropEvent(
    toIssue: any,
    relationType: IssueRelationType = IssueRelationType.Duplicates,
    subType: IssueRelationSubType | null = null
): RelationDropEvent {
    return { toIssue, relationType, subType };
}

describe('IssueTableComponent drag handlers (TestBed)', () => {
    let comp: any;
    let mocks: any;

    beforeEach(async () => {
        localStorage.clear();
        const result = await createTableFixture();
        comp = result.comp;
        mocks = result.mocks;
    });

    // =========================================================================
    // onDragStart
    // =========================================================================

    describe('onDragStart', () => {
        it('sets isDragging and draggingFromIssue, writes dataTransfer', () => {
            const issue = makeIssue({ idIssuePublic: 5 } as any);
            const dataTransfer = { setData: vi.fn() };
            comp.onDragStart({ dataTransfer } as any, issue);
            expect(comp.isDragging()).toBe(true);
            expect(comp.draggingFromIssue()).toEqual(issue);
            expect(dataTransfer.setData).toHaveBeenCalledWith('text/plain', '5');
        });
    });

    // =========================================================================
    // onDragEnter
    // =========================================================================

    describe('onDragEnter', () => {
        it('prevents default and sets idIssueDragOver for a different issue', () => {
            const preventDefault = vi.fn();
            const fromIssue = makeIssue({ idIssuePublic: 1 } as any);
            const targetIssue = makeIssue({ idIssuePublic: 2 } as any);
            comp.isDragging.set(true);
            comp.draggingFromIssue.set(fromIssue);
            comp.onDragEnter({ preventDefault } as any, targetIssue);
            expect(preventDefault).toHaveBeenCalled();
            expect(comp.idIssueDragOver()).toBe(2);
        });

        it('no-op when not dragging', () => {
            const preventDefault = vi.fn();
            comp.isDragging.set(false);
            comp.onDragEnter({ preventDefault } as any, makeIssue({ idIssuePublic: 2 } as any));
            expect(preventDefault).not.toHaveBeenCalled();
        });

        it('no-op when entering the same issue being dragged', () => {
            const preventDefault = vi.fn();
            const fromIssue = makeIssue({ idIssuePublic: 1 } as any);
            comp.isDragging.set(true);
            comp.draggingFromIssue.set(fromIssue);
            comp.onDragEnter({ preventDefault } as any, fromIssue);
            expect(preventDefault).not.toHaveBeenCalled();
        });
    });

    // =========================================================================
    // onDragEnd
    // =========================================================================

    // =========================================================================
    // onDragLeave
    // =========================================================================

    describe('onDragLeave', () => {
        beforeEach(() => vi.useFakeTimers());
        afterEach(() => vi.useRealTimers());

        it('suppresses when suppressNextDragLeave is true', () => {
            comp.suppressNextDragLeave = true;
            comp.idIssueDragOver.set(5);
            comp.onDragLeave({} as any);
            expect(comp.suppressNextDragLeave).toBe(false);
            expect(comp.idIssueDragOver()).toBe(5);
        });

        it('does not clear when relatedTarget is inside a drop-zone-row', () => {
            comp.idIssueDragOver.set(5);
            const dropZoneEl = document.createElement('div');
            dropZoneEl.className = 'drop-zone-row';
            comp.onDragLeave({ relatedTarget: dropZoneEl } as any);
            expect(comp.idIssueDragOver()).toBe(5);
        });

        it('clears idIssueDragOver after 80ms timeout', () => {
            comp.idIssueDragOver.set(5);
            comp.onDragLeave({ relatedTarget: null } as any);
            expect(comp.idIssueDragOver()).toBe(5);
            vi.advanceTimersByTime(80);
            expect(comp.idIssueDragOver()).toBeNull();
        });
    });

    describe('onDragEnd', () => {
        it('resets all drag state', () => {
            comp.isDragging.set(true);
            comp.draggingFromIssue.set(makeIssue() as any);
            comp.idIssueDragOver.set(5);
            comp.onDragEnd();
            expect(comp.isDragging()).toBe(false);
            expect(comp.draggingFromIssue()).toBeNull();
            expect(comp.idIssueDragOver()).toBeNull();
        });
    });
});

// =========================================================================
// onDropZone + Lag dialog
// =========================================================================

describe('IssueTableComponent onDropZone (TestBed)', () => {
    let comp: any;
    let mocks: any;

    beforeEach(async () => {
        localStorage.clear();
        const result = await createTableFixture();
        comp = result.comp;
        mocks = result.mocks;
    });

    describe('schedule relation with askLag enabled', () => {
        it('opens lag dialog and stores pending relation', () => {
            const fromIssue = makeIssue({ idIssuePublic: 1 } as any);
            const toIssue = makeIssue({ idIssuePublic: 2 } as any);
            comp.isAskLag.set(true);
            comp.draggingFromIssue.set(fromIssue);
            comp.onDropZone(
                makeDropEvent(
                    toIssue,
                    IssueRelationType.Schedule,
                    IssueRelationSubType.FinishToStart
                )
            );
            expect(comp.showLagDialog()).toBe(true);
            expect(comp.pendingRelation).toEqual({
                from: fromIssue,
                to: toIssue,
                relationType: IssueRelationType.Schedule,
                subType: IssueRelationSubType.FinishToStart
            });
        });
    });

    describe('non-schedule or askLag disabled', () => {
        it('submits relation directly without lag dialog', () => {
            const submitSpy = vi.fn();
            const fromIssue = makeIssue({ idIssuePublic: 1 } as any);
            const toIssue = makeIssue({ idIssuePublic: 2 } as any);
            comp.isAskLag.set(false);
            comp.draggingFromIssue.set(fromIssue);
            comp.submitRelation = submitSpy;
            comp.onDropZone(makeDropEvent(toIssue, IssueRelationType.Duplicates, null));
            expect(submitSpy).toHaveBeenCalledWith(
                fromIssue,
                toIssue,
                IssueRelationType.Duplicates,
                null,
                null
            );
            expect(comp.showLagDialog()).toBe(false);
        });

        it('schedule with askLag disabled submits directly', () => {
            const submitSpy = vi.fn();
            const fromIssue = makeIssue({ idIssuePublic: 1 } as any);
            const toIssue = makeIssue({ idIssuePublic: 2 } as any);
            comp.isAskLag.set(false);
            comp.draggingFromIssue.set(fromIssue);
            comp.submitRelation = submitSpy;
            comp.onDropZone(
                makeDropEvent(
                    toIssue,
                    IssueRelationType.Schedule,
                    IssueRelationSubType.FinishToStart
                )
            );
            expect(submitSpy).toHaveBeenCalledWith(
                fromIssue,
                toIssue,
                IssueRelationType.Schedule,
                IssueRelationSubType.FinishToStart,
                null
            );
        });

        it('non-schedule with askLag enabled submits directly (lag only for schedule)', () => {
            const submitSpy = vi.fn();
            const fromIssue = makeIssue({ idIssuePublic: 1 } as any);
            const toIssue = makeIssue({ idIssuePublic: 2 } as any);
            comp.isAskLag.set(true);
            comp.draggingFromIssue.set(fromIssue);
            comp.submitRelation = submitSpy;
            comp.onDropZone(
                makeDropEvent(toIssue, IssueRelationType.Hierarchy, IssueRelationSubType.Child)
            );
            expect(submitSpy).toHaveBeenCalledWith(
                fromIssue,
                toIssue,
                IssueRelationType.Hierarchy,
                IssueRelationSubType.Child,
                null
            );
            expect(comp.showLagDialog()).toBe(false);
        });
    });

    describe('without draggingFromIssue', () => {
        it('no-op when draggingFromIssue is null', () => {
            const submitSpy = vi.fn();
            comp.draggingFromIssue.set(null);
            comp.submitRelation = submitSpy;
            comp.onDropZone(makeDropEvent(makeIssue({ idIssuePublic: 2 } as any)));
            expect(submitSpy).not.toHaveBeenCalled();
        });
    });
});

describe('IssueTableComponent lag dialog (TestBed)', () => {
    let comp: any;

    beforeEach(async () => {
        localStorage.clear();
        const result = await createTableFixture();
        comp = result.comp;
    });

    describe('onConfirmLag', () => {
        it('submits relation with lag minutes and closes dialog', () => {
            const submitSpy = vi.fn();
            const fromIssue = makeIssue({ idIssuePublic: 1 } as any);
            const toIssue = makeIssue({ idIssuePublic: 2 } as any);
            comp.pendingRelation = {
                from: fromIssue,
                to: toIssue,
                relationType: IssueRelationType.Schedule,
                subType: IssueRelationSubType.FinishToStart
            };
            comp.lagMinutes.set(30);
            comp.showLagDialog.set(true);
            comp.submitRelation = submitSpy;

            comp.onConfirmLag();

            expect(submitSpy).toHaveBeenCalledWith(
                fromIssue,
                toIssue,
                IssueRelationType.Schedule,
                IssueRelationSubType.FinishToStart,
                30
            );
            expect(comp.showLagDialog()).toBe(false);
            expect(comp.lagMinutes()).toBeNull();
            expect(comp.pendingRelation).toBeNull();
        });

        it('no-op when pendingRelation is null', () => {
            const submitSpy = vi.fn();
            comp.pendingRelation = null;
            comp.submitRelation = submitSpy;
            comp.onConfirmLag();
            expect(submitSpy).not.toHaveBeenCalled();
        });

        it('submits with null lag when lagMinutes is null', () => {
            const submitSpy = vi.fn();
            const fromIssue = makeIssue({ idIssuePublic: 1 } as any);
            const toIssue = makeIssue({ idIssuePublic: 2 } as any);
            comp.pendingRelation = {
                from: fromIssue,
                to: toIssue,
                relationType: IssueRelationType.Schedule,
                subType: null
            };
            comp.lagMinutes.set(null);
            comp.showLagDialog.set(true);
            comp.submitRelation = submitSpy;

            comp.onConfirmLag();
            expect(submitSpy).toHaveBeenCalledWith(
                fromIssue,
                toIssue,
                IssueRelationType.Schedule,
                null,
                null
            );
        });
    });

    describe('onCancelLag', () => {
        it('closes dialog, clears lagMinutes and pendingRelation', () => {
            comp.pendingRelation = {
                from: makeIssue() as any,
                to: makeIssue() as any,
                relationType: 'schedule' as any,
                subType: null
            };
            comp.lagMinutes.set(30);
            comp.showLagDialog.set(true);

            comp.onCancelLag();

            expect(comp.showLagDialog()).toBe(false);
            expect(comp.lagMinutes()).toBeNull();
            expect(comp.pendingRelation).toBeNull();
        });
    });
});

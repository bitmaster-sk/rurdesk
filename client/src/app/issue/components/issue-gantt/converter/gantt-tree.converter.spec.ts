import { describe, expect, it } from 'vitest';
import { GanttTreeConverter } from './gantt-tree.converter';
import type { GanttRelation } from '../model/gantt-relation.model';
import { IssueRelationType } from '../../../constants/issue-relation-type.enum';
import { IssueRelationDirection } from '../../../constants/issue-relation-direction.enum';

const hierarchy = (
    idParent: number,
    idChild: number,
    direction = IssueRelationDirection.Outbound
): GanttRelation => ({
    idIssueRelation: idParent * 100 + idChild,
    relationType: IssueRelationType.Hierarchy,
    relationSubType: null,
    lagMinutes: null,
    direction,
    label: '',
    inverseLabel: '',
    from: { idIssuePublic: idParent },
    to: { idIssuePublic: idChild },
    createdAt: '',
    createdBy: 0
});

const tasks = (...ids: number[]): { idIssuePublic: number }[] =>
    ids.map(idIssuePublic => ({ idIssuePublic }));

describe('GanttTreeConverter.parentMap', () => {
    it('keeps only outbound hierarchy edges between loaded tasks', () => {
        const parents = GanttTreeConverter.parentMap(
            [hierarchy(1, 2), hierarchy(1, 3, IssueRelationDirection.Inbound), hierarchy(9, 4)],
            new Set([1, 2, 3, 4])
        );
        expect([...parents]).toEqual([[2, 1]]);
    });

    it('drops the edge that would close a cycle', () => {
        const parents = GanttTreeConverter.parentMap(
            [hierarchy(1, 2), hierarchy(2, 3), hierarchy(3, 1)],
            new Set([1, 2, 3])
        );
        expect(parents.get(2)).toBe(1);
        expect(parents.get(3)).toBe(2);
        expect(parents.has(1)).toBe(false);
    });
});

describe('GanttTreeConverter.preorder', () => {
    it('puts children right after their parent and keeps sibling order', () => {
        const parents = new Map([
            [3, 1],
            [4, 1],
            [5, 2]
        ]);
        expect(GanttTreeConverter.preorder([4, 2, 1, 3, 5], parents)).toEqual([2, 5, 1, 4, 3]);
    });
});

describe('GanttTreeConverter.visibleRows', () => {
    const parents = new Map([
        [2, 1],
        [3, 1],
        [4, 3],
        [6, 5]
    ]);
    const list = tasks(1, 2, 3, 4, 5, 6);

    it('numbers rows like a WBS and indents by depth', () => {
        const rows = GanttTreeConverter.visibleRows(list, parents, {
            collapsed: new Set(),
            outlineLevel: null
        });
        expect(rows.map(r => [r.task.idIssuePublic, r.wbsCode, r.depth, r.hasChildren])).toEqual([
            [1, '1', 0, true],
            [2, '1.1', 1, false],
            [3, '1.2', 1, true],
            [4, '1.2.1', 2, false],
            [5, '2', 0, true],
            [6, '2.1', 1, false]
        ]);
    });

    it('hides the subtree of a collapsed row and counts what it hides', () => {
        const rows = GanttTreeConverter.visibleRows(list, parents, {
            collapsed: new Set([1]),
            outlineLevel: null
        });
        expect(rows.map(r => r.task.idIssuePublic)).toEqual([1, 5, 6]);
        expect(rows[0].isCollapsed).toBe(true);
        expect(rows[0].hiddenCount).toBe(3);
    });

    it('collapses everything below the outline level', () => {
        const rows = GanttTreeConverter.visibleRows(list, parents, {
            collapsed: new Set(),
            outlineLevel: 2
        });
        expect(rows.map(r => r.wbsCode)).toEqual(['1', '1.1', '1.2', '2', '2.1']);
        expect(rows.find(r => r.wbsCode === '1.2')?.isCollapsed).toBe(true);
    });

    it('marks context rows', () => {
        const rows = GanttTreeConverter.visibleRows(list, parents, {
            collapsed: new Set(),
            outlineLevel: null,
            contextIds: new Set([1])
        });
        expect(rows.filter(r => r.isContext).map(r => r.task.idIssuePublic)).toEqual([1]);
    });
});

describe('GanttTreeConverter outline targets', () => {
    const parents = new Map([
        [2, 1],
        [3, 1],
        [4, 1]
    ]);
    const order = [1, 2, 3, 4, 5];

    it('indents under the previous sibling', () => {
        expect(GanttTreeConverter.indentTarget(3, order, parents)).toBe(2);
        expect(GanttTreeConverter.indentTarget(5, order, parents)).toBe(1);
    });

    it('does nothing for the first sibling', () => {
        expect(GanttTreeConverter.indentTarget(2, order, parents)).toBeNull();
        expect(GanttTreeConverter.indentTarget(1, order, parents)).toBeNull();
    });

    it('outdents to the grandparent, after the old parent', () => {
        expect(GanttTreeConverter.outdentTarget(3, parents)).toEqual({
            idParent: null,
            afterId: 1
        });
        expect(GanttTreeConverter.outdentTarget(1, parents)).toBeNull();
    });
});

describe('GanttTreeConverter drag and drop', () => {
    it('maps the pointer to a zone of the row', () => {
        expect(GanttTreeConverter.dropZone(0.1)).toBe('before');
        expect(GanttTreeConverter.dropZone(0.5)).toBe('into');
        expect(GanttTreeConverter.dropZone(0.9)).toBe('after');
    });

    it('refuses a drop into the own subtree', () => {
        const parents = new Map([
            [2, 1],
            [3, 2]
        ]);
        expect(GanttTreeConverter.canDrop(1, 3, parents)).toBe(false);
        expect(GanttTreeConverter.canDrop(1, 1, parents)).toBe(false);
        expect(GanttTreeConverter.canDrop(3, 1, parents)).toBe(true);
    });

    it('reorders siblings around the target', () => {
        expect(GanttTreeConverter.reorderSiblings([1, 2, 3], 3, 1, 'before')).toEqual([3, 1, 2]);
        expect(GanttTreeConverter.reorderSiblings([1, 2, 3], 1, 2, 'after')).toEqual([2, 1, 3]);
        expect(GanttTreeConverter.reorderSiblings([1, 2], 9, 1, 'after')).toEqual([1, 9, 2]);
    });
});

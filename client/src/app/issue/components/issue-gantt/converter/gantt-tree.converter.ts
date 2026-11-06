import type { GanttRelation } from '../model/gantt-relation.model';
import { IssueRelationDirection } from '../../../constants/issue-relation-direction.enum';
import { IssueRelationType } from '../../../constants/issue-relation-type.enum';
import { DROP_EDGE_FRACTION } from '../constants/gantt-tree.constants';
import {
    GanttDropZone,
    GanttOutdentTarget,
    GanttTreeRow,
    GanttVisibleRowsOptions,
    ParentMap
} from '../entity/gantt-tree.entity';

export abstract class GanttTreeConverter {
    public static parentMap(relations: GanttRelation[], ids: ReadonlySet<number>): ParentMap {
        const parents: ParentMap = new Map();
        for (const relation of relations) {
            if (relation.relationType !== IssueRelationType.Hierarchy) continue;
            if (relation.direction !== IssueRelationDirection.Outbound) continue;
            const idParent = relation.from.idIssuePublic;
            const idChild = relation.to.idIssuePublic;
            if (!ids.has(idParent) || !ids.has(idChild) || idParent === idChild) continue;
            if (parents.has(idChild)) continue;
            if (GanttTreeConverter.isAncestor(idChild, idParent, parents)) continue;
            parents.set(idChild, idParent);
        }
        return parents;
    }

    public static isAncestor(idAncestor: number, id: number, parents: ParentMap): boolean {
        const seen = new Set<number>();
        let current = parents.get(id);
        while (current !== undefined && !seen.has(current)) {
            if (current === idAncestor) return true;
            seen.add(current);
            current = parents.get(current);
        }
        return false;
    }

    public static childrenMap(order: number[], parents: ParentMap): Map<number | null, number[]> {
        const rank = new Map(order.map((id, index) => [id, index]));
        const children = new Map<number | null, number[]>();
        for (const id of order) {
            const idParent = parents.get(id) ?? null;
            const list = children.get(idParent) ?? [];
            list.push(id);
            children.set(idParent, list);
        }
        for (const list of children.values()) {
            list.sort((a, b) => rank.get(a)! - rank.get(b)!);
        }
        return children;
    }

    public static preorder(order: number[], parents: ParentMap): number[] {
        const children = GanttTreeConverter.childrenMap(order, parents);
        const result: number[] = [];
        const walk = (idParent: number | null): void => {
            for (const id of children.get(idParent) ?? []) {
                result.push(id);
                walk(id);
            }
        };
        walk(null);
        return result;
    }

    public static descendants(id: number, children: Map<number | null, number[]>): number[] {
        const result: number[] = [];
        const walk = (idParent: number): void => {
            for (const idChild of children.get(idParent) ?? []) {
                result.push(idChild);
                walk(idChild);
            }
        };
        walk(id);
        return result;
    }

    public static visibleRows<T extends { idIssuePublic: number }>(
        tasks: T[],
        parents: ParentMap,
        options: GanttVisibleRowsOptions
    ): GanttTreeRow<T>[] {
        const byId = new Map(tasks.map(t => [t.idIssuePublic, t]));
        const children = GanttTreeConverter.childrenMap(
            tasks.map(t => t.idIssuePublic),
            parents
        );
        const rows: GanttTreeRow<T>[] = [];
        const walk = (idParent: number | null, depth: number, prefix: string): void => {
            (children.get(idParent) ?? []).forEach((id, index) => {
                const wbsCode = prefix ? `${prefix}.${index + 1}` : `${index + 1}`;
                const kids = children.get(id) ?? [];
                const hasChildren = kids.length > 0;
                const isCollapsed =
                    hasChildren &&
                    (options.collapsed.has(id) ||
                        (options.outlineLevel !== null && depth + 1 >= options.outlineLevel));
                rows.push({
                    task: byId.get(id)!,
                    depth,
                    idParent,
                    hasChildren,
                    isCollapsed,
                    hiddenCount: isCollapsed
                        ? GanttTreeConverter.descendants(id, children).length
                        : 0,
                    wbsCode,
                    isContext: options.contextIds?.has(id) ?? false
                });
                if (!isCollapsed) walk(id, depth + 1, wbsCode);
            });
        };
        walk(null, 0, '');
        return rows;
    }

    public static withContextBeforeDescendants(
        order: number[],
        contextIds: number[],
        parents: ParentMap
    ): number[] {
        const result = [...order];
        const depth = (id: number): number => {
            let d = 0;
            for (
                let p = parents.get(id);
                p !== undefined && d < order.length + contextIds.length;
                p = parents.get(p)
            )
                d++;
            return d;
        };
        const deepestFirst = [...contextIds].sort((a, b) => depth(b) - depth(a));
        for (const id of deepestFirst) {
            const index = result.findIndex(other =>
                GanttTreeConverter.isAncestor(id, other, parents)
            );
            if (index < 0) result.push(id);
            else result.splice(index, 0, id);
        }
        return result;
    }

    public static reorderSiblings(
        siblings: number[],
        idMoved: number,
        idTarget: number,
        zone: Exclude<GanttDropZone, 'into'>
    ): number[] {
        const result = siblings.filter(id => id !== idMoved);
        const index = result.indexOf(idTarget);
        if (index < 0) return [...result, idMoved];
        result.splice(zone === 'before' ? index : index + 1, 0, idMoved);
        return result;
    }

    public static siblings(
        id: number,
        order: number[],
        parents: ParentMap
    ): { idParent: number | null; ids: number[] } {
        const idParent = parents.get(id) ?? null;
        const children = GanttTreeConverter.childrenMap(order, parents);
        return { idParent, ids: children.get(idParent) ?? [] };
    }

    public static indentTarget(id: number, order: number[], parents: ParentMap): number | null {
        const { ids } = GanttTreeConverter.siblings(id, order, parents);
        const index = ids.indexOf(id);
        return index > 0 ? ids[index - 1] : null;
    }

    public static outdentTarget(id: number, parents: ParentMap): GanttOutdentTarget | null {
        const idParent = parents.get(id);
        if (idParent === undefined) return null;
        return { idParent: parents.get(idParent) ?? null, afterId: idParent };
    }

    public static dropZone(fraction: number): GanttDropZone {
        if (fraction < DROP_EDGE_FRACTION) return 'before';
        if (fraction > 1 - DROP_EDGE_FRACTION) return 'after';
        return 'into';
    }

    public static canDrop(idMoved: number, idTarget: number, parents: ParentMap): boolean {
        return idMoved !== idTarget && !GanttTreeConverter.isAncestor(idMoved, idTarget, parents);
    }
}

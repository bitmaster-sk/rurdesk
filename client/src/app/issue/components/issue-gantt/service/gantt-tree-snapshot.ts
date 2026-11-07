import { Issue } from '../../../model/issue.model';
import type { ParentMap } from '../entity/gantt-tree.entity';
import { TaskSnapshot } from '../entity/gantt-cascade.entity';

export class GanttTreeSnapshot {
    public readonly tasks = new Map<number, TaskSnapshot>();
    public readonly summaryShifts = new Map<number, number>();
    private readonly original = new Map<number, number>();
    private readonly children = new Map<number, number[]>();
    private readonly summaries = new Set<number>();
    private readonly parents: ParentMap;
    private readonly partial: ReadonlySet<number>;

    public constructor(
        allTasks: Issue[],
        parents: ParentMap,
        partial: ReadonlySet<number> = new Set()
    ) {
        this.parents = parents;
        this.partial = partial;
        for (const task of allTasks) {
            if (!task.scheduledAt) continue;
            this.tasks.set(task.idIssuePublic, {
                idIssuePublic: task.idIssuePublic,
                scheduledAt: new Date(task.scheduledAt),
                estimated: task.estimated ?? 3600
            });
            this.original.set(task.idIssuePublic, task.scheduledAt.getTime());
            if (task.isSummary) this.summaries.add(task.idIssuePublic);
        }
        for (const [idChild, idParent] of parents) {
            if (!this.tasks.has(idChild) || !this.tasks.has(idParent)) continue;
            const list = this.children.get(idParent) ?? [];
            list.push(idChild);
            this.children.set(idParent, list);
        }
    }

    public descendants(id: number): number[] {
        const result: number[] = [];
        for (const idChild of this.children.get(id) ?? []) {
            result.push(idChild, ...this.descendants(idChild));
        }
        return result;
    }

    public leaves(id: number): number[] {
        if (!this.children.has(id)) return this.summaries.has(id) ? [] : [id];
        return this.descendants(id).filter(d => !this.children.has(d));
    }

    public onSameBranch(a: number, b: number): boolean {
        return a === b || this.ancestors(a).includes(b) || this.ancestors(b).includes(a);
    }

    public ancestors(id: number): number[] {
        const result: number[] = [];
        let current = this.parents.get(id);
        while (current !== undefined && this.tasks.has(current) && !result.includes(current)) {
            result.push(current);
            current = this.parents.get(current);
        }
        return result;
    }

    public span(id: number): TaskSnapshot | null {
        const own = this.tasks.get(id);
        if (!own) return null;
        const leaves = this.leaves(id);
        if (!this.children.has(id) || leaves.length === 0) return own;
        let start = Infinity;
        let end = -Infinity;
        for (const leaf of leaves) {
            const snapshot = this.tasks.get(leaf)!;
            start = Math.min(start, snapshot.scheduledAt.getTime());
            end = Math.max(end, snapshot.scheduledAt.getTime() + snapshot.estimated * 1000);
        }
        if (this.partial.has(id)) {
            start = Math.min(start, own.scheduledAt.getTime());
            end = Math.max(end, own.scheduledAt.getTime() + own.estimated * 1000);
        }
        return { idIssuePublic: id, scheduledAt: new Date(start), estimated: (end - start) / 1000 };
    }

    public shift(id: number, ms: number): number[] {
        const moved = this.leaves(id);
        for (const leaf of moved) {
            const snapshot = this.tasks.get(leaf)!;
            snapshot.scheduledAt = new Date(snapshot.scheduledAt.getTime() + ms);
        }
        for (const node of [id, ...this.descendants(id)]) {
            if (moved.includes(node)) continue;
            if (!this.partial.has(node) && this.children.has(node)) continue;
            const own = this.tasks.get(node)!;
            own.scheduledAt = new Date(own.scheduledAt.getTime() + ms);
        }
        if (this.summaries.has(id) || this.children.has(id)) {
            this.summaryShifts.set(id, (this.summaryShifts.get(id) ?? 0) + ms);
        }
        return moved;
    }

    public changedLeaves(): Map<number, { scheduledAt: Date }> {
        const changed = new Map<number, { scheduledAt: Date }>();
        for (const [id, snapshot] of this.tasks) {
            if (this.children.has(id) || this.summaries.has(id)) continue;
            if (snapshot.scheduledAt.getTime() !== this.original.get(id)) {
                changed.set(id, { scheduledAt: snapshot.scheduledAt });
            }
        }
        return changed;
    }
}

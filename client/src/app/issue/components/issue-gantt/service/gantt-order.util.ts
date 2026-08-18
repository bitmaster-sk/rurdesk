import type { Issue } from '../../../model/issue.model';
import type { GanttRelation } from '../model/gantt-relation.model';
import { IssueRelationDirection } from '../../../constants/issue-relation-direction.enum';

/**
 * applyPendingOrder reorders `tasks` to match `order` (a list of idIssuePublic)
 * for the optimistic drag overlay. Falls back to `tasks` unchanged when the
 * overlay is stale — i.e. the id SET differs (a row was added or removed).
 */
export function applyPendingOrder<T extends { idIssuePublic?: number }>(
    tasks: T[],
    order: number[] | null
): T[] {
    if (!order) return tasks;
    const byId = new Map(tasks.map(t => [t.idIssuePublic, t]));
    if (order.length !== tasks.length || order.some(id => !byId.has(id))) {
        return tasks;
    }
    return order.map(id => byId.get(id)!);
}

/**
 * Topological sort using Kahn's algorithm — O(V + E) time complexity.
 * V = number of issues, E = number of schedule relations.
 * For typical project sizes (< 1000 issues), this completes in < 1ms.
 * Falls back to scheduledAt order for unconnected components.
 */
export function topologicalSort<T extends Issue>(issues: T[], relations: GanttRelation[]): T[] {
    const issueMap = new Map(issues.map(i => [i.idIssuePublic, i]));
    const adjacency = new Map<number, number[]>();
    const inDegree = new Map<number, number>();

    for (const issue of issues) {
        adjacency.set(issue.idIssuePublic, []);
        inDegree.set(issue.idIssuePublic, 0);
    }

    for (const relation of relations) {
        if (relation.direction !== IssueRelationDirection.Outbound) continue;
        const fromId = relation.from.idIssuePublic;
        const toId = relation.to.idIssuePublic;
        if (!adjacency.has(fromId) || !adjacency.has(toId)) continue;

        adjacency.get(fromId)!.push(toId);
        inDegree.set(toId, (inDegree.get(toId) ?? 0) + 1);
    }

    // Kahn's algorithm
    const queue: number[] = [];
    for (const [id, degree] of inDegree) {
        if (degree === 0) queue.push(id);
    }
    // Sort initial queue by scheduledAt, then title as tiebreaker
    queue.sort((a, b) => {
        const issueA = issueMap.get(a);
        const issueB = issueMap.get(b);
        const dateA = issueA?.scheduledAt?.getTime() ?? 0;
        const dateB = issueB?.scheduledAt?.getTime() ?? 0;
        return dateA !== dateB
            ? dateA - dateB
            : (issueA?.title ?? '').localeCompare(issueB?.title ?? '');
    });

    const sorted: T[] = [];
    while (queue.length > 0) {
        const currentId = queue.shift()!;
        const issue = issueMap.get(currentId);
        if (issue) sorted.push(issue);

        const neighbors = adjacency.get(currentId) ?? [];
        for (const neighborId of neighbors) {
            const newDegree = (inDegree.get(neighborId) ?? 1) - 1;
            inDegree.set(neighborId, newDegree);
            if (newDegree === 0) {
                const neighbor = issueMap.get(neighborId);
                const neighborDate = neighbor?.scheduledAt?.getTime() ?? 0;
                const neighborTitle = neighbor?.title ?? '';
                const insertAt = queue.findIndex(id => {
                    const qIssue = issueMap.get(id);
                    const qDate = qIssue?.scheduledAt?.getTime() ?? 0;
                    return qDate !== neighborDate
                        ? qDate > neighborDate
                        : (qIssue?.title ?? '').localeCompare(neighborTitle) > 0;
                });
                if (insertAt === -1) {
                    queue.push(neighborId);
                } else {
                    queue.splice(insertAt, 0, neighborId);
                }
            }
        }
    }

    // Append any issues not reached (disconnected) sorted by scheduledAt
    const sortedSet = new Set(sorted.map(i => i.idIssuePublic));
    const remaining = issues
        .filter(i => !sortedSet.has(i.idIssuePublic))
        .sort((a, b) => (a.scheduledAt?.getTime() ?? 0) - (b.scheduledAt?.getTime() ?? 0));
    sorted.push(...remaining);

    return sorted;
}

/**
 * orderScheduled sorts by manual `ganttRank` when any issue has one; otherwise it
 * preserves today's dependency-driven topological order. Null-rank issues (newly
 * scheduled, not yet placed) fall to the bottom, tie-broken by scheduledAt then title.
 */
export function orderScheduled<T extends Issue>(issues: T[], relations: GanttRelation[]): T[] {
    const hasRank = issues.some(i => i.ganttRank != null);
    if (!hasRank) {
        return topologicalSort(issues, relations);
    }
    return [...issues].sort((a, b) => {
        const ra = a.ganttRank ?? null;
        const rb = b.ganttRank ?? null;
        if (ra != null && rb != null) return ra < rb ? -1 : ra > rb ? 1 : 0;
        if (ra != null) return -1; // ranked before unranked
        if (rb != null) return 1;
        const da = a.scheduledAt?.getTime() ?? 0;
        const db = b.scheduledAt?.getTime() ?? 0;
        return da !== db ? da - db : (a.title ?? '').localeCompare(b.title ?? '');
    });
}

import { Injectable } from '@angular/core';
import { GanttRelation } from '../model/gantt-relation.model';
import { Issue } from '../../../model/issue.model';
import { IssueRelationType } from '../../../constants/issue-relation-type.enum';

export interface CriticalPathResult {
    taskIds: Set<number>;
    relationIds: Set<number>;
    /** Position of each critical relation along the chain, 0 = at the source.
     *  Drives the sequential "trace" animation when the overlay turns on. */
    relationOrder: Map<number, number>;
}

/** Single definition of "no critical path" — reused by the disabled state. */
export function emptyCriticalPath(): CriticalPathResult {
    return { taskIds: new Set(), relationIds: new Set(), relationOrder: new Map() };
}

@Injectable()
export class GanttCriticalPathService {
    /**
     * Computes the critical path using a CPM forward pass.
     * The longest path from any source (no inbound) to any sink (no outbound).
     * Edge weight: from.estimated + lagMinutes. Final node adds its own estimated.
     */
    public computeCriticalPath(tasks: Issue[], relations: GanttRelation[]): CriticalPathResult {
        const emptyResult = emptyCriticalPath();

        if (tasks.length === 0) return emptyResult;

        const taskMap = new Map(tasks.map(t => [t.idIssuePublic, t]));

        // Build adjacency (outbound only)
        const outbound = new Map<
            number,
            { toId: number; lagMinutes: number; relationId: number }[]
        >();
        const inbound = new Map<number, number[]>();

        for (const task of tasks) {
            outbound.set(task.idIssuePublic, []);
            inbound.set(task.idIssuePublic, []);
        }

        for (const relation of relations) {
            if (relation.relationType !== IssueRelationType.Schedule) continue;
            if (relation.direction !== 'outbound') continue;
            const fromId = relation.from.idIssuePublic;
            const toId = relation.to.idIssuePublic;
            if (!taskMap.has(fromId) || !taskMap.has(toId)) continue;

            outbound.get(fromId)!.push({
                toId,
                lagMinutes: relation.lagMinutes ?? 0,
                relationId: relation.idIssueRelation
            });
            inbound.get(toId)!.push(fromId);
        }

        // Find source nodes (no inbound edges)
        const sources = tasks.filter(t => (inbound.get(t.idIssuePublic) ?? []).length === 0);

        // Forward pass: compute earliest start time for each task
        const earliestStart = new Map<number, number>(); // taskId → earliest start in seconds
        const predecessor = new Map<number, { fromId: number; relationId: number } | null>();

        // Topological order via Kahn's
        const inDegree = new Map<number, number>();
        for (const task of tasks) {
            inDegree.set(task.idIssuePublic, (inbound.get(task.idIssuePublic) ?? []).length);
            earliestStart.set(task.idIssuePublic, 0);
            predecessor.set(task.idIssuePublic, null);
        }

        const queue = sources.map(t => t.idIssuePublic);

        while (queue.length > 0) {
            const currentId = queue.shift()!;
            const currentTask = taskMap.get(currentId)!;
            const currentEarliest = earliestStart.get(currentId) ?? 0;
            const currentEstimated = currentTask.estimated ?? 3600;

            for (const edge of outbound.get(currentId) ?? []) {
                const edgeWeight = currentEstimated + edge.lagMinutes * 60;
                const candidateStart = currentEarliest + edgeWeight;

                if (candidateStart > (earliestStart.get(edge.toId) ?? 0)) {
                    earliestStart.set(edge.toId, candidateStart);
                    predecessor.set(edge.toId, {
                        fromId: currentId,
                        relationId: edge.relationId
                    });
                }

                const newDegree = (inDegree.get(edge.toId) ?? 1) - 1;
                inDegree.set(edge.toId, newDegree);
                if (newDegree === 0) queue.push(edge.toId);
            }
        }

        // Find the sink (task with no outbound) with the greatest earliest_start + estimated
        const sinks = tasks.filter(t => (outbound.get(t.idIssuePublic) ?? []).length === 0);
        if (sinks.length === 0) return emptyResult;

        let maxEndTime = 0;
        let criticalSinkId: number | null = null;

        for (const sink of sinks) {
            const endTime = (earliestStart.get(sink.idIssuePublic) ?? 0) + (sink.estimated ?? 3600);
            if (endTime > maxEndTime) {
                maxEndTime = endTime;
                criticalSinkId = sink.idIssuePublic;
            }
        }

        if (criticalSinkId === null) return emptyResult;

        // Trace back from critical sink to find the critical path
        const criticalTaskIds = new Set<number>();
        const criticalRelationIds = new Set<number>();
        const chainedRelationIds: number[] = []; // sink → source while tracing back

        let currentId: number | null = criticalSinkId;
        while (currentId !== null) {
            criticalTaskIds.add(currentId);
            const pred = predecessor.get(currentId);
            if (pred) {
                criticalRelationIds.add(pred.relationId);
                chainedRelationIds.push(pred.relationId);
                currentId = pred.fromId;
            } else {
                currentId = null;
            }
        }

        const relationOrder = new Map<number, number>();
        chainedRelationIds.reverse().forEach((id, index) => relationOrder.set(id, index));

        return {
            taskIds: criticalTaskIds,
            relationIds: criticalRelationIds,
            relationOrder
        };
    }
}

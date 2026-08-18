import { Injectable } from '@angular/core';
import { GanttRelation } from '../model/gantt-relation.model';
import { Issue } from '../../../model/issue.model';
import { IssueRelationType } from '../../../constants/issue-relation-type.enum';
import { IssueRelationSubType } from '../../../constants/issue-relation-subtype.enum';
import { IssueRelationDirection } from '../../../constants/issue-relation-direction.enum';
import { addSeconds } from 'date-fns';

export interface CascadeResult {
    affectedTasks: Map<number, { scheduledAt: Date }>;
}

interface TaskSnapshot {
    idIssuePublic: number;
    scheduledAt: Date;
    estimated: number;
}

@Injectable()
export class GanttCascadeService {
    /**
     * Computes cascading scheduledAt shifts through the schedule DAG.
     *
     * @param changedTaskId - The task that was moved or resized.
     * @param newScheduledAt - New scheduledAt for the changed task (move), or current scheduledAt (resize).
     * @param newEstimated - New estimated for the changed task (resize), or current estimated (move).
     * @param allTasks - All scheduled tasks.
     * @param relations - All schedule relations (outbound direction).
     */
    public computeCascade(
        changedTaskId: number,
        newScheduledAt: Date,
        newEstimated: number,
        allTasks: Issue[],
        relations: GanttRelation[]
    ): CascadeResult {
        const taskMap = new Map<number, TaskSnapshot>();
        for (const task of allTasks) {
            if (task.scheduledAt) {
                taskMap.set(task.idIssuePublic, {
                    idIssuePublic: task.idIssuePublic,
                    scheduledAt: new Date(task.scheduledAt),
                    estimated: task.estimated ?? 3600
                });
            }
        }

        // Apply the initial change
        const changedSnapshot = taskMap.get(changedTaskId);
        if (changedSnapshot) {
            changedSnapshot.scheduledAt = newScheduledAt;
            changedSnapshot.estimated = newEstimated;
        }

        // Build adjacency: from → [{ toId, subType, lagMinutes }]
        const outbound = new Map<
            number,
            {
                toId: number;
                subType: IssueRelationSubType;
                lagMinutes: number;
            }[]
        >();

        for (const relation of relations) {
            if (relation.relationType !== IssueRelationType.Schedule) continue;
            if (relation.direction !== IssueRelationDirection.Outbound) continue;
            const fromId = relation.from.idIssuePublic;
            const toId = relation.to.idIssuePublic;
            const subType = relation.relationSubType ?? IssueRelationSubType.FinishToStart;
            const lagMinutes = relation.lagMinutes ?? 0;

            if (!outbound.has(fromId)) outbound.set(fromId, []);
            outbound.get(fromId)!.push({ toId, subType, lagMinutes });
        }

        // BFS cascade — no visited guard; the constraint check
        // `dependent.scheduledAt < requiredStart` prevents re-processing when
        // there is no improvement. Since schedule relations form a DAG
        // (enforced by cycle detection), this always terminates.
        const affected = new Map<number, { scheduledAt: Date }>();
        const queue: number[] = [changedTaskId];

        while (queue.length > 0) {
            const currentId = queue.shift()!;

            const currentTask = taskMap.get(currentId);
            if (!currentTask) continue;

            const edges = outbound.get(currentId) ?? [];
            for (const edge of edges) {
                const dependent = taskMap.get(edge.toId);
                if (!dependent) continue;

                const requiredStart = this.computeConstraint(
                    currentTask,
                    dependent,
                    edge.subType,
                    edge.lagMinutes
                );

                if (requiredStart && dependent.scheduledAt < requiredStart) {
                    dependent.scheduledAt = requiredStart;
                    affected.set(edge.toId, { scheduledAt: requiredStart });
                    queue.push(edge.toId);
                }
            }
        }

        return { affectedTasks: affected };
    }

    /**
     * Returns the minimum scheduledAt the dependent must have to satisfy the constraint.
     * Returns null if no constraint violation is possible.
     */
    private computeConstraint(
        from: TaskSnapshot,
        to: TaskSnapshot,
        subType: IssueRelationSubType,
        lagMinutes: number
    ): Date | null {
        const fromStart = from.scheduledAt.getTime();
        const fromEnd = addSeconds(from.scheduledAt, from.estimated).getTime();
        const lagMs = lagMinutes * 60 * 1000;
        const toEstimatedMs = to.estimated * 1000;

        switch (subType) {
            case IssueRelationSubType.FinishToStart:
                // to.scheduledAt >= from.scheduledAt + from.estimated + lag
                return new Date(fromEnd + lagMs);

            case IssueRelationSubType.StartToStart:
                // to.scheduledAt >= from.scheduledAt + lag
                return new Date(fromStart + lagMs);

            case IssueRelationSubType.FinishToFinish:
                // to.scheduledAt >= from.scheduledAt + from.estimated - to.estimated + lag
                return new Date(fromEnd - toEstimatedMs + lagMs);

            case IssueRelationSubType.StartToFinish:
                // to.scheduledAt >= from.scheduledAt + lag - to.estimated
                return new Date(fromStart + lagMs - toEstimatedMs);

            default:
                return null;
        }
    }
}

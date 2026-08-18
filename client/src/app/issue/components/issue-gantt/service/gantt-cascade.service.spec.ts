import { GanttCascadeService } from './gantt-cascade.service';
import { Issue } from '../../../model/issue.model';
import { ReadIssueRelationDto } from '../../../model/issue-relation.model';
import { IssueRelationType } from '../../../constants/issue-relation-type.enum';
import { IssueRelationDirection } from '../../../constants/issue-relation-direction.enum';
import { IssueRelationSubType } from '../../../constants/issue-relation-subtype.enum';
// Issue is imported for Pick<Issue, 'idIssuePublic'> used in makeRelation

describe('GanttCascadeService', () => {
    let service: GanttCascadeService;

    beforeEach(() => {
        service = new GanttCascadeService();
    });

    function makeIssue(id: number, scheduledAt: string, estimated: number): Issue {
        return {
            idIssuePublic: id,
            idProject: 1,
            idState: null,
            idSeverity: null,
            title: `Issue ${id}`,
            description: '',
            tracked: 0,
            estimated,
            scheduledAt: new Date(scheduledAt)
        };
    }

    function makeRelation(
        fromId: number,
        toId: number,
        subType: IssueRelationSubType,
        lagMinutes?: number
    ): ReadIssueRelationDto {
        return {
            idIssueRelation: 1,
            relationType: IssueRelationType.Schedule,
            relationSubType: subType,
            lagMinutes: lagMinutes ?? null,
            direction: IssueRelationDirection.Outbound,
            label: '',
            inverseLabel: '',
            from: {
                idIssuePublic: fromId,
                title: `Issue ${fromId}`,
                idSeverity: null,
                idState: null,
                assignedTo: null,
                updateAt: '',
                qualityScore: null
            },
            to: {
                idIssuePublic: toId,
                title: `Issue ${toId}`,
                idSeverity: null,
                idState: null,
                assignedTo: null,
                updateAt: '',
                qualityScore: null
            },
            createdAt: '',
            createdBy: 1
        };
    }

    it('finish_to_start: should cascade dependent forward', () => {
        const tasks = [
            makeIssue(1, '2026-04-10T09:00:00Z', 3600), // 1h
            makeIssue(2, '2026-04-10T10:00:00Z', 3600) // starts right after
        ];
        const relations = [makeRelation(1, 2, IssueRelationSubType.FinishToStart)];

        // Move task 1 to 11:00 → task 2 must move to 12:00
        const result = service.computeCascade(
            1,
            new Date('2026-04-10T11:00:00Z'),
            3600,
            tasks,
            relations
        );

        const task2New = result.affectedTasks.get(2);
        expect(task2New).toBeDefined();
        expect(task2New!.scheduledAt.toISOString()).toBe('2026-04-10T12:00:00.000Z');
    });

    it('finish_to_start: should NOT cascade if already satisfied', () => {
        const tasks = [
            makeIssue(1, '2026-04-10T09:00:00Z', 3600),
            makeIssue(2, '2026-04-10T14:00:00Z', 3600) // already well ahead
        ];
        const relations = [makeRelation(1, 2, IssueRelationSubType.FinishToStart)];

        // Move task 1 to 08:00 (earlier) → task 2 already satisfies constraint
        const result = service.computeCascade(
            1,
            new Date('2026-04-10T08:00:00Z'),
            3600,
            tasks,
            relations
        );

        expect(result.affectedTasks.size).toBe(0);
    });

    it('should cascade through chain A → B → C', () => {
        const tasks = [
            makeIssue(1, '2026-04-10T09:00:00Z', 3600),
            makeIssue(2, '2026-04-10T10:00:00Z', 3600),
            makeIssue(3, '2026-04-10T11:00:00Z', 3600)
        ];
        const relations = [
            makeRelation(1, 2, IssueRelationSubType.FinishToStart),
            makeRelation(2, 3, IssueRelationSubType.FinishToStart)
        ];

        // Move task 1 to 12:00 → task 2 moves to 13:00, task 3 to 14:00
        const result = service.computeCascade(
            1,
            new Date('2026-04-10T12:00:00Z'),
            3600,
            tasks,
            relations
        );

        expect(result.affectedTasks.get(2)!.scheduledAt.toISOString()).toBe(
            '2026-04-10T13:00:00.000Z'
        );
        expect(result.affectedTasks.get(3)!.scheduledAt.toISOString()).toBe(
            '2026-04-10T14:00:00.000Z'
        );
    });

    it('start_to_start: should apply lag correctly', () => {
        const tasks = [
            makeIssue(1, '2026-04-10T09:00:00Z', 3600),
            makeIssue(2, '2026-04-10T09:30:00Z', 3600)
        ];
        const relations = [makeRelation(1, 2, IssueRelationSubType.StartToStart, 60)]; // 60min lag

        // Move task 1 to 10:00 → task 2 must be at 11:00 (start + 60min)
        const result = service.computeCascade(
            1,
            new Date('2026-04-10T10:00:00Z'),
            3600,
            tasks,
            relations
        );

        expect(result.affectedTasks.get(2)!.scheduledAt.toISOString()).toBe(
            '2026-04-10T11:00:00.000Z'
        );
    });

    it('finish_to_finish: should compute correctly', () => {
        const tasks = [
            makeIssue(1, '2026-04-10T09:00:00Z', 7200), // 2h, ends at 11:00
            makeIssue(2, '2026-04-10T09:00:00Z', 3600) // 1h, ends at 10:00
        ];
        const relations = [makeRelation(1, 2, IssueRelationSubType.FinishToFinish)];

        // to.scheduledAt >= from.end - to.estimated = 11:00 - 1h = 10:00
        // task 2 is at 09:00 which < 10:00 → must move to 10:00
        const result = service.computeCascade(
            1,
            new Date('2026-04-10T09:00:00Z'),
            7200,
            tasks,
            relations
        );

        expect(result.affectedTasks.get(2)!.scheduledAt.toISOString()).toBe(
            '2026-04-10T10:00:00.000Z'
        );
    });

    it('start_to_finish: should compute correctly', () => {
        const tasks = [
            makeIssue(1, '2026-04-10T09:00:00Z', 3600),
            makeIssue(2, '2026-04-10T07:00:00Z', 3600) // 1h task
        ];
        const relations = [makeRelation(1, 2, IssueRelationSubType.StartToFinish)];

        // StartToFinish: successor must FINISH no earlier than predecessor START + lag.
        //   to.scheduledAt >= from.scheduledAt + lag - to.estimated = 09:00 + 0 - 1h = 08:00
        // task 2 currently starts 07:00 (finishes 08:00 < 09:00) → violated, so it shifts to
        // start 08:00 (finishes 09:00). One task is affected.
        const result = service.computeCascade(
            1,
            new Date('2026-04-10T09:00:00Z'),
            3600,
            tasks,
            relations
        );

        expect(result.affectedTasks.size).toBe(1);
        expect(result.affectedTasks.get(2)!.scheduledAt.toISOString()).toBe(
            '2026-04-10T08:00:00.000Z'
        );
    });
});

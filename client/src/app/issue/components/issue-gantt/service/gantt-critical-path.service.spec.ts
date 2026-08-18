import { GanttCriticalPathService } from './gantt-critical-path.service';
import { Issue } from '../../../model/issue.model';
import { ReadIssueRelationDto } from '../../../model/issue-relation.model';
import { IssueRelationType } from '../../../constants/issue-relation-type.enum';
import { IssueRelationDirection } from '../../../constants/issue-relation-direction.enum';
import { IssueRelationSubType } from '../../../constants/issue-relation-subtype.enum';

describe('GanttCriticalPathService', () => {
    let service: GanttCriticalPathService;

    beforeEach(() => {
        service = new GanttCriticalPathService();
    });

    function makeIssue(id: number, estimated: number): Issue {
        return {
            idIssue: id,
            idIssuePublic: id,
            idProject: 1,
            idState: null,
            idSeverity: null,
            title: `Issue ${id}`,
            description: '',
            tracked: 0,
            estimated,
            scheduledAt: new Date('2026-04-10T09:00:00Z')
        };
    }

    function makeRelation(
        id: number,
        fromId: number,
        toId: number,
        lag?: number
    ): ReadIssueRelationDto {
        return {
            idIssueRelation: id,
            relationType: IssueRelationType.Schedule,
            relationSubType: IssueRelationSubType.FinishToStart,
            lagMinutes: lag ?? null,
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

    it('simple chain A → B → C: all on critical path', () => {
        const tasks = [makeIssue(1, 3600), makeIssue(2, 3600), makeIssue(3, 3600)];
        const relations = [makeRelation(1, 1, 2), makeRelation(2, 2, 3)];

        const result = service.computeCriticalPath(tasks, relations);

        expect(result.taskIds.has(1)).toBe(true);
        expect(result.taskIds.has(2)).toBe(true);
        expect(result.taskIds.has(3)).toBe(true);
        expect(result.relationIds.size).toBe(2);
    });

    it('parallel paths: only longest is critical', () => {
        // A(1h) → C(1h)    total: 2h
        // B(3h) → C(1h)    total: 4h  ← critical
        const tasks = [makeIssue(1, 3600), makeIssue(2, 10800), makeIssue(3, 3600)];
        const relations = [makeRelation(1, 1, 3), makeRelation(2, 2, 3)];

        const result = service.computeCriticalPath(tasks, relations);

        expect(result.taskIds.has(2)).toBe(true); // B is on critical path
        expect(result.taskIds.has(3)).toBe(true); // C is on critical path
        expect(result.taskIds.has(1)).toBe(false); // A is NOT on critical path
    });

    it('single node with no relations is its own critical path', () => {
        const tasks = [makeIssue(1, 3600)];
        const result = service.computeCriticalPath(tasks, []);

        expect(result.taskIds.has(1)).toBe(true);
        expect(result.taskIds.size).toBe(1);
    });

    it('lag minutes included in path weight', () => {
        // A(1h) --lag 2h--> B(1h) total: 1h + 2h(lag) + 1h = 4h  ← critical
        // C(2h) ----------> B(1h) total: 2h + 1h = 3h
        const tasks = [makeIssue(1, 3600), makeIssue(2, 7200), makeIssue(3, 3600)];
        const relations = [makeRelation(1, 1, 3, 120), makeRelation(2, 2, 3)];

        const result = service.computeCriticalPath(tasks, relations);

        expect(result.taskIds.has(1)).toBe(true);
        expect(result.taskIds.has(3)).toBe(true);
    });
});

import { describe, expect, it } from 'vitest';
import { orderScheduled, applyPendingOrder } from './gantt-order.util';
import type { Issue } from '../../../model/issue.model';

const iss = (id: number, rank: string | null, scheduledAt: string, title: string): Issue =>
    ({
        idIssuePublic: id,
        idProject: 1,
        idState: null,
        idSeverity: null,
        title,
        description: '',
        tracked: 0,
        scheduledAt: new Date(scheduledAt),
        ganttRank: rank
    }) as Issue;

describe('orderScheduled', () => {
    it('sorts by ganttRank ascending when ranks are present', () => {
        const out = orderScheduled(
            [iss(1, 't', '2026-01-02', 'A'), iss(2, 'm', '2026-01-01', 'B')],
            []
        );
        expect(out.map(i => i.idIssuePublic)).toEqual([2, 1]);
    });

    it('places null-rank issues last, tie-broken by scheduledAt', () => {
        const out = orderScheduled(
            [
                iss(1, null, '2026-01-05', 'A'),
                iss(2, 'm', '2026-01-01', 'B'),
                iss(3, null, '2026-01-03', 'C')
            ],
            []
        );
        expect(out.map(i => i.idIssuePublic)).toEqual([2, 3, 1]);
    });

    it('falls back to topological (scheduledAt) order when no rank is set', () => {
        const out = orderScheduled(
            [iss(1, null, '2026-01-05', 'A'), iss(2, null, '2026-01-01', 'B')],
            []
        );
        expect(out.map(i => i.idIssuePublic)).toEqual([2, 1]);
    });
});

describe('applyPendingOrder', () => {
    const t = (id: number) => ({ idIssuePublic: id });

    it('reorders tasks to match the pending order when the id set matches', () => {
        const out = applyPendingOrder([t(1), t(2), t(3)], [3, 1, 2]);
        expect(out.map(x => x.idIssuePublic)).toEqual([3, 1, 2]);
    });

    it('returns tasks unchanged when there is no pending order', () => {
        const tasks = [t(1), t(2)];
        expect(applyPendingOrder(tasks, null)).toBe(tasks);
    });

    it('falls back to server order when the id set differs (row added/removed)', () => {
        const tasks = [t(1), t(2), t(4)]; // 4 appeared; overlay still references 3
        const out = applyPendingOrder(tasks, [3, 1, 2]);
        expect(out.map(x => x.idIssuePublic)).toEqual([1, 2, 4]);
    });
});

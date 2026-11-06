import { describe, expect, it } from 'vitest';
import { Issue } from '../model/issue.model';
import { TagPresence } from '../constants/tag-presence.enum';
import { BulkEditPlan } from './bulk-edit-plan';

const issue = (idIssuePublic: number, extra: Partial<Issue> = {}): Issue => ({
    idIssue: idIssuePublic,
    idIssuePublic,
    idProject: 1,
    idState: 1,
    idSeverity: null,
    idIssueType: null,
    title: '',
    description: '',
    tracked: 0,
    ...extra
});

describe('BulkEditPlan', () => {
    const issues = [issue(1, { idsTag: [10, 20] }), issue(2, { idsTag: [10] }), issue(3)];

    it('tells for each tag whether all, some or none of the selected tasks carry it', () => {
        const presenceByTag = BulkEditPlan.initialTagPresence(issues.slice(0, 2), [10, 20, 30]);
        expect(presenceByTag.get(10)).toBe(TagPresence.All);
        expect(presenceByTag.get(20)).toBe(TagPresence.Some);
        expect(presenceByTag.get(30)).toBe(TagPresence.None);
    });

    it('cycles a mixed tag through keep, add to all, remove from all', () => {
        expect(BulkEditPlan.nextTagPresence(TagPresence.Some, TagPresence.Some)).toBe(
            TagPresence.All
        );
        expect(BulkEditPlan.nextTagPresence(TagPresence.All, TagPresence.Some)).toBe(
            TagPresence.None
        );
        expect(BulkEditPlan.nextTagPresence(TagPresence.None, TagPresence.Some)).toBe(
            TagPresence.Some
        );
    });

    it('cycles a tag every task has, or none has, between all and none', () => {
        expect(BulkEditPlan.nextTagPresence(TagPresence.All, TagPresence.All)).toBe(
            TagPresence.None
        );
        expect(BulkEditPlan.nextTagPresence(TagPresence.None, TagPresence.All)).toBe(
            TagPresence.All
        );
        expect(BulkEditPlan.nextTagPresence(TagPresence.None, TagPresence.None)).toBe(
            TagPresence.All
        );
    });

    it('turns only changed tag presence into tag additions and removals', () => {
        const initial = new Map<number, TagPresence>([
            [10, TagPresence.All],
            [20, TagPresence.Some],
            [30, TagPresence.None]
        ]);
        const current = new Map<number, TagPresence>([
            [10, TagPresence.All],
            [20, TagPresence.None],
            [30, TagPresence.All]
        ]);
        expect(BulkEditPlan.tagChanges(initial, current)).toEqual({
            addIdsTag: [30],
            removeIdsTag: [20]
        });
        expect(BulkEditPlan.tagChanges(initial, initial)).toEqual({});
    });

    it('builds one entry per task carrying the same changes', () => {
        const entries = BulkEditPlan.toEntries(issues.slice(0, 2), {
            idState: 4,
            idUserAssigned: null,
            addIdsTag: [30]
        });
        expect(entries).toEqual([
            { idIssuePublic: 1, idState: 4, idUserAssigned: null, addIdsTag: [30] },
            { idIssuePublic: 2, idState: 4, idUserAssigned: null, addIdsTag: [30] }
        ]);
    });

    it('leaves the computed dates of a summary task out', () => {
        const summary = { ...issues[0], isSummary: true };
        const entries = BulkEditPlan.toEntries([summary, issues[1]], {
            scheduledAt: '2026-11-02T08:00:00.000Z',
            idState: 4
        });
        expect(entries).toEqual([
            { idIssuePublic: 1, idState: 4 },
            { idIssuePublic: 2, scheduledAt: '2026-11-02T08:00:00.000Z', idState: 4 }
        ]);
    });

    it('drops a summary task whose only change is its dates', () => {
        const summary = { ...issues[0], isSummary: true };
        const entries = BulkEditPlan.toEntries([summary], { estimated: 3600 });
        expect(entries).toEqual([]);
    });
});

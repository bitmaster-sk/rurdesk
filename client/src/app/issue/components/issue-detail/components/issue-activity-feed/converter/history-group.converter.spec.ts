import { IssueHistoryField } from 'src/app/issue/constants/issue-history-field.enum';
import { IssueHistoryLookups } from 'src/app/issue/entity/issue-history-group.entity';
import { IssueHistory, IssueHistoryUser } from 'src/app/issue/model/issue-history.model';
import { IssueType } from 'src/app/issue-type/model/issue-type.model';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { Fixtures } from 'src/testing/fixtures';
import { HistoryGroupConverter } from './history-group.converter';

function row(overrides: Partial<IssueHistory>): IssueHistory {
    return {
        idIssueHistory: 1,
        idChange: 'a',
        field: IssueHistoryField.Title,
        oldValue: 'old',
        newValue: 'new',
        idUser: 7,
        source: 'user',
        createAt: '2026-11-10T08:00:00Z',
        ...overrides
    };
}

const NEW_STATE: IssueState = Fixtures.state({ idState: 1, idProject: 10, name: 'New' });
const HIGH: IssueSeverity = Fixtures.severity({ idSeverity: 2, title: 'High', color: '#f00' });
const BUG: IssueType = Fixtures.issueType({ idIssueType: 3, name: 'Bug' });
const ALICE: IssueHistoryUser = {
    idUser: 7,
    name: 'Alice',
    colorAvatarBg: '',
    avatarInitials: null,
    isActive: true
};

const lookups: IssueHistoryLookups = {
    states: new Map([[1, NEW_STATE]]),
    severities: new Map([[2, HIGH]]),
    issueTypes: new Map([[3, BUG]]),
    users: new Map([[7, ALICE]]),
    tags: new Map([
        [4, Fixtures.tag({ idTag: 4, name: 'backend', color: '#00f' })],
        [5, Fixtures.tag({ idTag: 5, name: 'frontend', color: '#0f0' })]
    ])
};

describe('HistoryGroupConverter.toGroups', () => {
    it('groups rows by change and orders fields like the enum', () => {
        const groups = HistoryGroupConverter.toGroups([
            row({ idIssueHistory: 1, idChange: 'a', field: IssueHistoryField.Title }),
            row({ idIssueHistory: 2, idChange: 'a', field: IssueHistoryField.State }),
            row({
                idIssueHistory: 3,
                idChange: 'b',
                field: IssueHistoryField.Estimated,
                idUser: null,
                source: 'system',
                createAt: '2026-11-10T09:00:00Z'
            })
        ]);

        expect(groups.map(group => group.idChange)).toEqual(['a', 'b']);
        expect(groups[0].changes.map(change => change.field)).toEqual([
            IssueHistoryField.State,
            IssueHistoryField.Title
        ]);
        expect(groups[1].source).toBe('system');
        expect(groups[1].idUser).toBeNull();
    });

    it('dates a group by its newest row', () => {
        const groups = HistoryGroupConverter.toGroups([
            row({ idChange: 'a', createAt: '2026-11-10T08:00:00Z' }),
            row({ idChange: 'a', field: IssueHistoryField.State, createAt: '2026-11-10T08:10:00Z' })
        ]);

        expect(groups[0].date.toISOString()).toBe('2026-11-10T08:10:00.000Z');
    });

    it('orders groups by date even when the input is not ordered', () => {
        const groups = HistoryGroupConverter.toGroups([
            row({ idChange: 'late', createAt: '2026-11-10T10:00:00Z' }),
            row({ idChange: 'early', createAt: '2026-11-10T07:00:00Z' })
        ]);

        expect(groups.map(group => group.idChange)).toEqual(['early', 'late']);
    });
});

describe('HistoryGroupConverter.toValue', () => {
    const value = (field: IssueHistoryField, raw: unknown) =>
        HistoryGroupConverter.toValue(field, raw, lookups);

    it('shows a field specific placeholder for an empty value', () => {
        expect(value(IssueHistoryField.Assignee, null)).toEqual({
            kind: 'empty',
            labelKey: 'ISSUE.HISTORY.EMPTY.ASSIGNEE'
        });
        expect(value(IssueHistoryField.State, null)).toEqual({
            kind: 'empty',
            labelKey: 'ISSUE.HISTORY.EMPTY.STATE'
        });
        expect(value(IssueHistoryField.RollupTracked, null)).toEqual({
            kind: 'empty',
            labelKey: 'ISSUE.HISTORY.EMPTY.DEFAULT'
        });
    });

    it('treats a zero estimate as no estimate', () => {
        expect(value(IssueHistoryField.Estimated, 0)).toEqual({
            kind: 'empty',
            labelKey: 'ISSUE.HISTORY.EMPTY.ESTIMATED'
        });
    });

    it('resolves ids to the objects the badges and avatar need', () => {
        expect(value(IssueHistoryField.State, 1)).toEqual({ kind: 'state', state: NEW_STATE });
        expect(value(IssueHistoryField.Severity, 2)).toEqual({ kind: 'severity', severity: HIGH });
        expect(value(IssueHistoryField.IssueType, 3)).toEqual({
            kind: 'issueType',
            issueType: BUG
        });
        expect(value(IssueHistoryField.Assignee, 7)).toEqual({ kind: 'user', user: ALICE });
    });

    it('shows #id for an id that no longer exists', () => {
        expect(value(IssueHistoryField.State, 99)).toEqual({ kind: 'text', text: '#99' });
        expect(value(IssueHistoryField.Assignee, 98)).toEqual({ kind: 'text', text: '#98' });
    });

    it('formats seconds like the estimate elsewhere', () => {
        expect(value(IssueHistoryField.Estimated, 5400)).toEqual({ kind: 'text', text: '1h 30m' });
        expect(value(IssueHistoryField.RollupTracked, 3600)).toEqual({ kind: 'text', text: '1h' });
    });

    it('formats the scheduled date with time', () => {
        const raw = new Date(2026, 10, 10, 8, 30).toISOString();
        expect(value(IssueHistoryField.ScheduledAt, raw)).toEqual({
            kind: 'text',
            text: 'Nov 10, 2026 08:30'
        });
    });

    it('returns translation keys for the task kind', () => {
        expect(value(IssueHistoryField.IsSummary, true)).toEqual({
            kind: 'translated',
            labelKey: 'ISSUE.HISTORY.SUMMARY_ON'
        });
        expect(value(IssueHistoryField.IsSummary, false)).toEqual({
            kind: 'translated',
            labelKey: 'ISSUE.HISTORY.SUMMARY_OFF'
        });
    });

    it('prints plain values as they are', () => {
        expect(value(IssueHistoryField.Title, 'Hello')).toEqual({ kind: 'text', text: 'Hello' });
        expect(value(IssueHistoryField.Points, 5)).toEqual({ kind: 'text', text: '5' });
    });
});

describe('HistoryGroupConverter.toTagChanges', () => {
    it('lists only the added and removed tags', () => {
        expect(HistoryGroupConverter.toTagChanges([4], [4, 5], lookups.tags)).toEqual({
            added: [{ idTag: 5, name: 'frontend', color: '#0f0' }],
            removed: []
        });
        expect(HistoryGroupConverter.toTagChanges([4], [5], lookups.tags)).toEqual({
            added: [{ idTag: 5, name: 'frontend', color: '#0f0' }],
            removed: [{ idTag: 4, name: 'backend', color: '#00f' }]
        });
    });

    it('names a deleted tag by its id', () => {
        expect(HistoryGroupConverter.toTagChanges([], [97], lookups.tags)).toEqual({
            added: [{ idTag: 97, name: '#97', color: null }],
            removed: []
        });
    });
});

describe('HistoryGroupConverter.toRows', () => {
    it('turns a group into a description row, a tag row and value rows', () => {
        const [group] = HistoryGroupConverter.toGroups([
            row({ field: IssueHistoryField.Description, oldValue: 'a', newValue: 'b' }),
            row({ field: IssueHistoryField.Tags, oldValue: [], newValue: [4] }),
            row({ field: IssueHistoryField.Assignee, oldValue: null, newValue: 7 })
        ]);

        expect(HistoryGroupConverter.toRows(group, lookups)).toEqual([
            {
                kind: 'value',
                labelKey: 'ISSUE.HISTORY.FIELD.ASSIGNEE',
                oldValue: { kind: 'empty', labelKey: 'ISSUE.HISTORY.EMPTY.ASSIGNEE' },
                newValue: { kind: 'user', user: ALICE }
            },
            { kind: 'description' },
            {
                kind: 'tags',
                labelKey: 'ISSUE.HISTORY.FIELD.TAGS',
                added: [{ idTag: 4, name: 'backend', color: '#00f' }],
                removed: []
            }
        ]);
    });
});

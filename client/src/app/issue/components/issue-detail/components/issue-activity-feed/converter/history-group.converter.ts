import { format } from 'date-fns';
import { IssueHistoryField } from 'src/app/issue/constants/issue-history-field.enum';
import {
    HistoryRow,
    HistoryTag,
    HistoryValue,
    IssueHistoryGroup,
    IssueHistoryLookups
} from 'src/app/issue/entity/issue-history-group.entity';
import { IssueHistory } from 'src/app/issue/model/issue-history.model';
import { DurationConverter } from 'src/app/shared/duration/duration.converter';
import { DurationFormatter } from 'src/app/shared/duration/duration.formatter';
import { Tag } from 'src/app/tag/model/tag.model';

const FIELD_ORDER: IssueHistoryField[] = Object.values(IssueHistoryField);

const EMPTY_KEYS: Partial<Record<IssueHistoryField, string>> = {
    [IssueHistoryField.State]: 'ISSUE.HISTORY.EMPTY.STATE',
    [IssueHistoryField.Severity]: 'ISSUE.HISTORY.EMPTY.SEVERITY',
    [IssueHistoryField.IssueType]: 'ISSUE.HISTORY.EMPTY.ISSUE_TYPE',
    [IssueHistoryField.Assignee]: 'ISSUE.HISTORY.EMPTY.ASSIGNEE',
    [IssueHistoryField.ScheduledAt]: 'ISSUE.HISTORY.EMPTY.SCHEDULED_AT',
    [IssueHistoryField.Estimated]: 'ISSUE.HISTORY.EMPTY.ESTIMATED',
    [IssueHistoryField.Points]: 'ISSUE.HISTORY.EMPTY.POINTS'
};

const DURATION_FIELDS = new Set<IssueHistoryField>([
    IssueHistoryField.Estimated,
    IssueHistoryField.RollupEstimated,
    IssueHistoryField.RollupTracked
]);

export abstract class HistoryGroupConverter {
    public static toGroups(rows: IssueHistory[]): IssueHistoryGroup[] {
        const groups = new Map<string, IssueHistoryGroup>();
        for (const row of rows) {
            const date = new Date(row.createAt);
            const group = groups.get(row.idChange);
            if (!group) {
                groups.set(row.idChange, {
                    idChange: row.idChange,
                    date,
                    idUser: row.idUser,
                    source: row.source,
                    changes: [row]
                });
                continue;
            }
            group.changes.push(row);
            if (date > group.date) {
                group.date = date;
            }
        }
        const result = [...groups.values()];
        for (const group of result) {
            group.changes.sort(
                (a, b) => FIELD_ORDER.indexOf(a.field) - FIELD_ORDER.indexOf(b.field)
            );
        }
        return result.sort((a, b) => a.date.getTime() - b.date.getTime());
    }

    public static toRows(group: IssueHistoryGroup, lookups: IssueHistoryLookups): HistoryRow[] {
        return group.changes.map(change => this.toRow(change, lookups));
    }

    public static toValue(
        field: IssueHistoryField,
        value: unknown,
        lookups: IssueHistoryLookups
    ): HistoryValue {
        if (this.isEmpty(field, value)) {
            return { kind: 'empty', labelKey: EMPTY_KEYS[field] ?? 'ISSUE.HISTORY.EMPTY.DEFAULT' };
        }
        const id = Number(value);
        switch (field) {
            case IssueHistoryField.State: {
                const state = lookups.states.get(id);
                return state ? { kind: 'state', state } : this.missing(id);
            }
            case IssueHistoryField.Severity: {
                const severity = lookups.severities.get(id);
                return severity ? { kind: 'severity', severity } : this.missing(id);
            }
            case IssueHistoryField.IssueType: {
                const issueType = lookups.issueTypes.get(id);
                return issueType ? { kind: 'issueType', issueType } : this.missing(id);
            }
            case IssueHistoryField.Assignee: {
                const user = lookups.users.get(id);
                return user ? { kind: 'user', user } : this.missing(id);
            }
            case IssueHistoryField.ScheduledAt:
                return {
                    kind: 'text',
                    text: format(new Date(value as string), 'MMM d, yyyy HH:mm')
                };
            case IssueHistoryField.IsSummary:
                return {
                    kind: 'translated',
                    labelKey: value ? 'ISSUE.HISTORY.SUMMARY_ON' : 'ISSUE.HISTORY.SUMMARY_OFF'
                };
            default:
                if (DURATION_FIELDS.has(field)) {
                    return {
                        kind: 'text',
                        text: DurationFormatter.durationToString(
                            DurationConverter.secondsToDuration(id)
                        )
                    };
                }
                return {
                    kind: 'text',
                    text: typeof value === 'object' ? JSON.stringify(value) : `${value as string}`
                };
        }
    }

    public static toTagChanges(
        oldValue: unknown,
        newValue: unknown,
        tags: Map<number, Tag>
    ): { added: HistoryTag[]; removed: HistoryTag[] } {
        const before = this.toIds(oldValue);
        const after = this.toIds(newValue);
        return {
            added: after.filter(id => !before.includes(id)).map(id => this.toTag(id, tags)),
            removed: before.filter(id => !after.includes(id)).map(id => this.toTag(id, tags))
        };
    }

    private static toRow(change: IssueHistory, lookups: IssueHistoryLookups): HistoryRow {
        if (change.field === IssueHistoryField.Description) {
            return { kind: 'description' };
        }
        const labelKey = `ISSUE.HISTORY.FIELD.${change.field.toUpperCase()}`;
        if (change.field === IssueHistoryField.Tags) {
            return {
                kind: 'tags',
                labelKey,
                ...this.toTagChanges(change.oldValue, change.newValue, lookups.tags)
            };
        }
        return {
            kind: 'value',
            labelKey,
            oldValue: this.toValue(change.field, change.oldValue, lookups),
            newValue: this.toValue(change.field, change.newValue, lookups)
        };
    }

    private static isEmpty(field: IssueHistoryField, value: unknown): boolean {
        if (value === null || value === undefined) {
            return true;
        }
        return DURATION_FIELDS.has(field) && Number(value) === 0;
    }

    private static missing(id: number): HistoryValue {
        return { kind: 'text', text: `#${id}` };
    }

    private static toIds(value: unknown): number[] {
        return Array.isArray(value) ? value.map(id => Number(id)) : [];
    }

    private static toTag(idTag: number, tags: Map<number, Tag>): HistoryTag {
        const tag = tags.get(idTag);
        return tag
            ? { idTag, name: tag.name, color: tag.color }
            : { idTag, name: `#${idTag}`, color: null };
    }
}

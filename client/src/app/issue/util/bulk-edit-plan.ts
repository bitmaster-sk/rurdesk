import { Issue } from '../model/issue.model';
import { BulkEditIssueChangesReq, BulkEditIssueEntryReq } from '../model/bulk-edit-issues.model';
import { TagPresence } from '../constants/tag-presence.enum';

export abstract class BulkEditPlan {
    public static initialTagPresence(
        issues: readonly Issue[],
        idsTag: readonly number[]
    ): Map<number, TagPresence> {
        const presenceByTag = new Map<number, TagPresence>();
        for (const idTag of idsTag) {
            const count = BulkEditPlan.countWithTag(issues, idTag);
            presenceByTag.set(
                idTag,
                count === 0
                    ? TagPresence.None
                    : count === issues.length
                      ? TagPresence.All
                      : TagPresence.Some
            );
        }
        return presenceByTag;
    }

    public static countWithTag(issues: readonly Issue[], idTag: number): number {
        return issues.filter(issue => issue.idsTag?.includes(idTag)).length;
    }

    public static nextTagPresence(current: TagPresence, initial: TagPresence): TagPresence {
        const cycle =
            initial === TagPresence.Some
                ? [TagPresence.Some, TagPresence.All, TagPresence.None]
                : [TagPresence.All, TagPresence.None];
        const index = cycle.indexOf(current);
        return cycle[(index + 1) % cycle.length];
    }

    public static tagChanges(
        initial: ReadonlyMap<number, TagPresence>,
        current: ReadonlyMap<number, TagPresence>
    ): Pick<BulkEditIssueChangesReq, 'addIdsTag' | 'removeIdsTag'> {
        const add: number[] = [];
        const remove: number[] = [];
        current.forEach((presence, idTag) => {
            if (presence === initial.get(idTag)) return;
            if (presence === TagPresence.All) add.push(idTag);
            if (presence === TagPresence.None) remove.push(idTag);
        });
        return {
            ...(add.length > 0 ? { addIdsTag: add } : {}),
            ...(remove.length > 0 ? { removeIdsTag: remove } : {})
        };
    }

    public static toEntries(
        issues: readonly Issue[],
        changes: BulkEditIssueChangesReq
    ): BulkEditIssueEntryReq[] {
        return issues.flatMap(issue => {
            if (!issue.isSummary) return [{ idIssuePublic: issue.idIssuePublic, ...changes }];
            const rest = { ...changes };
            delete rest.scheduledAt;
            delete rest.estimated;
            return Object.keys(rest).length > 0
                ? [{ idIssuePublic: issue.idIssuePublic, ...rest }]
                : [];
        });
    }
}

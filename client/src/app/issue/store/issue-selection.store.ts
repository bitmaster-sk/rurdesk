import { Injectable, computed, signal } from '@angular/core';
import { Issue } from '../model/issue.model';
import { IssuesFilter } from '../components/filter/issue-filter.entity';

export const BULK_SELECTION_LIMIT = 100;

export type SelectionResult = 'ok' | 'limit';

@Injectable()
export class IssueSelectionStore {
    private readonly selectedMap = signal<ReadonlyMap<number, Issue>>(new Map());
    private anchorId: number | null = null;
    private lastRange: readonly number[] = [];
    private scopeKey: string | null = null;

    public readonly selected = computed(() => [...this.selectedMap().values()]);
    public readonly count = computed(() => this.selectedMap().size);
    public readonly isLimitReached = computed(() => this.count() >= BULK_SELECTION_LIMIT);

    public isSelected(idIssuePublic: number): boolean {
        return this.selectedMap().has(idIssuePublic);
    }

    public toggle(issue: Issue): SelectionResult {
        this.anchorId = issue.idIssuePublic;
        this.lastRange = [];
        if (this.isSelected(issue.idIssuePublic)) {
            this.removeIds([issue.idIssuePublic]);
            return 'ok';
        }
        return this.add([issue]);
    }

    public selectRange(target: Issue, ordered: readonly Issue[]): SelectionResult {
        const range = this.rangeBetween(this.anchorId, target, ordered);
        if (!range) return this.toggle(target);
        return this.addRange(range);
    }

    public extendTo(
        target: Issue,
        ordered: readonly Issue[],
        fallbackAnchor: number | null
    ): SelectionResult {
        this.anchorId ??= fallbackAnchor ?? target.idIssuePublic;
        const range = this.rangeBetween(this.anchorId, target, ordered) ?? [target];
        this.removeIds(this.lastRange);
        return this.addRange(range);
    }

    public toggleAll(ordered: readonly Issue[]): SelectionResult {
        this.lastRange = [];
        if (ordered.length > 0 && ordered.every(issue => this.isSelected(issue.idIssuePublic))) {
            this.removeIds(ordered.map(issue => issue.idIssuePublic));
            return 'ok';
        }
        return this.add(ordered);
    }

    public clear(): void {
        this.anchorId = null;
        this.lastRange = [];
        if (this.selectedMap().size > 0) this.selectedMap.set(new Map());
    }

    public remove(idIssuePublic: number): void {
        if (this.anchorId === idIssuePublic) this.anchorId = null;
        this.lastRange = this.lastRange.filter(id => id !== idIssuePublic);
        if (this.isSelected(idIssuePublic)) this.removeIds([idIssuePublic]);
    }

    public refresh(issues: readonly Issue[]): void {
        const current = this.selectedMap();
        if (current.size === 0) return;
        let next: Map<number, Issue> | null = null;
        for (const issue of issues) {
            if (current.has(issue.idIssuePublic) && current.get(issue.idIssuePublic) !== issue) {
                next ??= new Map(current);
                next.set(issue.idIssuePublic, issue);
            }
        }
        if (next) this.selectedMap.set(next);
    }

    public clearOnScopeChange(filter: IssuesFilter): void {
        const key = JSON.stringify({ ...filter, orderColumn: null, orderDirection: null });
        if (this.scopeKey !== null && this.scopeKey !== key) this.clear();
        this.scopeKey = key;
    }

    private rangeBetween(
        anchorId: number | null,
        target: Issue,
        ordered: readonly Issue[]
    ): Issue[] | null {
        const from = ordered.findIndex(issue => issue.idIssuePublic === anchorId);
        const to = ordered.findIndex(issue => issue.idIssuePublic === target.idIssuePublic);
        if (from === -1 || to === -1) return null;
        return ordered.slice(Math.min(from, to), Math.max(from, to) + 1);
    }

    private addRange(range: readonly Issue[]): SelectionResult {
        const before = this.selectedMap();
        const result = this.add(range);
        this.lastRange = range
            .map(issue => issue.idIssuePublic)
            .filter(id => !before.has(id) && this.isSelected(id));
        return result;
    }

    private add(issues: readonly Issue[]): SelectionResult {
        const next = new Map(this.selectedMap());
        let result: SelectionResult = 'ok';
        for (const issue of issues) {
            if (next.has(issue.idIssuePublic)) continue;
            if (next.size >= BULK_SELECTION_LIMIT) {
                result = 'limit';
                break;
            }
            next.set(issue.idIssuePublic, issue);
        }
        this.selectedMap.set(next);
        return result;
    }

    private removeIds(ids: readonly number[]): void {
        if (ids.length === 0) return;
        const next = new Map(this.selectedMap());
        ids.forEach(id => next.delete(id));
        this.selectedMap.set(next);
    }
}

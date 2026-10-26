import { Injectable, inject, signal } from '@angular/core';
import { IssueApi } from '../api/issue.api.service';
import { IssueLinkOption } from '../entity/issue-link-option.entity';

/**
 * Caches the task list used by the `#123` autocomplete in message editors,
 * keyed by project (mirrors WikiLinkStore for `[[page]]` links). Loaded once
 * per project on first use; `#` links are blind, so the cache never has to be
 * validated.
 */
@Injectable({ providedIn: 'root' })
export class IssueLinkStore {
    private readonly api = inject(IssueApi);
    private readonly options = signal<ReadonlyMap<number, IssueLinkOption[]>>(new Map());
    private readonly idsLoading = new Set<number>();

    public list(idProject: number): IssueLinkOption[] | null {
        return this.options().get(idProject) ?? null;
    }

    public ensure(idProject: number): void {
        if (!this.options().has(idProject)) {
            this.reload(idProject);
        }
    }

    public reload(idProject: number): void {
        if (this.idsLoading.has(idProject)) {
            return;
        }
        this.idsLoading.add(idProject);
        this.api.load$({ idProject, orderColumn: 'createAt', orderDirection: 'desc' }).subscribe({
            next: issues => {
                this.idsLoading.delete(idProject);
                this.options.update(map =>
                    new Map(map).set(
                        idProject,
                        issues.map(issue => ({
                            idIssuePublic: issue.idIssuePublic,
                            title: issue.title
                        }))
                    )
                );
            },
            error: () => this.idsLoading.delete(idProject)
        });
    }
}

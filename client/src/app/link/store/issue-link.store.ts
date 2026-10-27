import { Injectable, inject, signal } from '@angular/core';
import { IssueApi } from 'src/app/issue/api/issue.api.service';
import { IssueLinkOption } from '../entity/issue-link-option.entity';

@Injectable({ providedIn: 'root' })
export class IssueLinkStore {
    private readonly api = inject(IssueApi);
    private readonly options = signal<ReadonlyMap<number, IssueLinkOption[]>>(new Map());
    private readonly idsLoading = new Set<number>();

    public list(idProject: number): IssueLinkOption[] | null {
        return this.options().get(idProject) ?? null;
    }

    public ensure(idProject: number): void {
        if (!this.idsLoading.has(idProject)) {
            this.reload(idProject);
        }
    }

    private reload(idProject: number): void {
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

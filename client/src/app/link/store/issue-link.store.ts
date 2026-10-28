import { Injectable, inject, signal } from '@angular/core';
import { combineLatest } from 'rxjs';
import { IssueApi } from 'src/app/issue/api/issue.api.service';
import { ProjectApi } from 'src/app/project/api/project.api.service';
import { Project } from 'src/app/project/model/project.model';
import { IssueLinkOption } from '../entity/issue-link-option.entity';

@Injectable({ providedIn: 'root' })
export class IssueLinkStore {
    private readonly api = inject(IssueApi);
    private readonly projectApi = inject(ProjectApi);
    private readonly options = signal<ReadonlyMap<number, IssueLinkOption[]>>(new Map());
    private readonly allOptions = signal<IssueLinkOption[] | null>(null);
    private readonly idsLoading = new Set<number>();
    private isAllLoading = false;

    public list(idProject: number): IssueLinkOption[] | null {
        return this.options().get(idProject) ?? null;
    }

    public listAll(): IssueLinkOption[] | null {
        return this.allOptions();
    }

    public ensure(idProject: number): void {
        if (!this.idsLoading.has(idProject)) {
            this.reload(idProject);
        }
    }

    public ensureAll(): void {
        if (!this.isAllLoading) {
            this.isAllLoading = true;
            this.projectApi.load$().subscribe({
                next: projects => this.reloadAll(projects),
                error: () => {
                    this.isAllLoading = false;
                }
            });
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
                            idProject,
                            title: issue.title
                        }))
                    )
                );
            },
            error: () => this.idsLoading.delete(idProject)
        });
    }

    private reloadAll(projects: Pick<Project, 'idProject' | 'name'>[]): void {
        if (projects.length === 0) {
            this.allOptions.set([]);
            this.isAllLoading = false;
            return;
        }
        combineLatest(
            projects.map(project =>
                this.api.load$({
                    idProject: project.idProject,
                    orderColumn: 'createAt',
                    orderDirection: 'desc'
                })
            )
        ).subscribe({
            next: lists => {
                this.allOptions.set(
                    projects.flatMap((project, index) =>
                        lists[index].map(issue => ({
                            idIssuePublic: issue.idIssuePublic,
                            idProject: project.idProject,
                            projectName: project.name,
                            title: issue.title
                        }))
                    )
                );
                this.isAllLoading = false;
            },
            error: () => {
                this.isAllLoading = false;
            }
        });
    }
}

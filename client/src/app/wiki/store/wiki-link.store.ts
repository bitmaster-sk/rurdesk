import { Injectable, inject, signal } from '@angular/core';
import { WikiApi } from '../api/wiki.api.service';
import { WikiTree } from '../model/wiki-tree.model';

@Injectable({ providedIn: 'root' })
export class WikiLinkStore {
    private readonly api = inject(WikiApi);
    private readonly trees = signal<ReadonlyMap<number, WikiTree>>(new Map());
    private readonly idsLoading = new Set<number>();

    public tree(idProject: number): WikiTree | null {
        return this.trees().get(idProject) ?? null;
    }

    public ensure(idProject: number): void {
        if (!this.trees().has(idProject)) {
            this.reload(idProject);
        }
    }

    public reload(idProject: number): void {
        if (this.idsLoading.has(idProject)) {
            return;
        }
        this.idsLoading.add(idProject);
        this.api.loadTree$(idProject).subscribe({
            next: tree => {
                this.idsLoading.delete(idProject);
                this.trees.update(trees => new Map(trees).set(idProject, tree));
            },
            error: () => this.idsLoading.delete(idProject)
        });
    }
}

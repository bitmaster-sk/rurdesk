import { Injectable, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Observable, debounceTime, filter, tap } from 'rxjs';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { WikiApi } from '../api/wiki.api.service';
import { WikiSpaceKind } from '../constants/wiki-space-kind.enum';
import { WikiTreeConverter } from '../converter/wiki-tree.converter';
import { WikiTreeGroups } from '../entity/wiki-tree-entry.entity';
import { WikiSpaceView, WikiTree } from '../model/wiki-tree.model';

@Injectable({ providedIn: 'root' })
export class WikiTreeStore {
    private readonly api = inject(WikiApi);

    private readonly treeState = signal<WikiTree | null>(null);
    private readonly expandedState = signal<ReadonlySet<number>>(new Set());
    private readonly projectState = signal<number | null>(null);

    public readonly tree = this.treeState.asReadonly();
    public readonly idProject = this.projectState.asReadonly();
    public readonly expanded = this.expandedState.asReadonly();

    public readonly groups = computed<WikiTreeGroups | null>(() => {
        const tree = this.treeState();
        return tree ? WikiTreeConverter.toGroups(tree) : null;
    });

    public readonly sharedSpace = computed<WikiSpaceView | null>(
        () => this.treeState()?.spaces.find(space => space.kind === WikiSpaceKind.Instance) ?? null
    );

    public readonly projectSpace = computed<WikiSpaceView | null>(
        () => this.treeState()?.spaces.find(space => space.kind === WikiSpaceKind.Project) ?? null
    );

    public constructor() {
        inject(NoticeService)
            .wikiPage$.pipe(
                filter(notice =>
                    (this.treeState()?.spaces ?? []).some(
                        space => space.idSpace === notice.payload.idSpace
                    )
                ),
                debounceTime(300),
                takeUntilDestroyed()
            )
            .subscribe(() => this.reload());
    }

    public load$(idProject: number): Observable<WikiTree> {
        if (this.projectState() !== idProject) {
            this.treeState.set(null);
            this.projectState.set(idProject);
            this.expandedState.set(this.readExpanded(idProject));
        }
        return this.api.loadTree$(idProject).pipe(tap(tree => this.treeState.set(tree)));
    }

    public reload(): void {
        const idProject = this.projectState();
        if (idProject !== null) {
            this.load$(idProject).subscribe();
        }
    }

    public kindOfSpace(idSpace: number): WikiSpaceKind {
        return this.sharedSpace()?.idSpace === idSpace
            ? WikiSpaceKind.Instance
            : WikiSpaceKind.Project;
    }

    public isExpanded(idPage: number): boolean {
        return this.expandedState().has(idPage);
    }

    public toggle(idPage: number): void {
        const next = new Set(this.expandedState());
        if (next.has(idPage)) {
            next.delete(idPage);
        } else {
            next.add(idPage);
        }
        this.setExpanded(next);
    }

    public expandPathTo(idPage: number): void {
        const tree = this.treeState();
        if (!tree) {
            return;
        }
        const ancestors = WikiTreeConverter.ancestorIds(tree.nodes, idPage);
        if (ancestors.every(id => this.expandedState().has(id))) {
            return;
        }
        this.setExpanded(new Set([...this.expandedState(), ...ancestors]));
    }

    private setExpanded(expanded: Set<number>): void {
        this.expandedState.set(expanded);
        const idProject = this.projectState();
        if (idProject !== null) {
            localStorage.setItem(this.storageKey(idProject), JSON.stringify([...expanded]));
        }
    }

    private readExpanded(idProject: number): ReadonlySet<number> {
        try {
            const raw = localStorage.getItem(this.storageKey(idProject));
            const ids: unknown = raw ? JSON.parse(raw) : [];
            return new Set(
                Array.isArray(ids) ? ids.filter((id): id is number => typeof id === 'number') : []
            );
        } catch {
            return new Set();
        }
    }

    private storageKey(idProject: number): string {
        return `wiki.expanded.${idProject}`;
    }
}

import { Injectable, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { NavigationEnd, Router } from '@angular/router';
import orderBy from 'lodash-es/orderBy';
import { filter } from 'rxjs';
import { ProjectApi } from 'src/app/project/api/project.api.service';
import { Project } from 'src/app/project/model/project.model';
import { ProjectStore } from 'src/app/project/project.store';

const LAST_PROJECT_KEY = 'lastProjectId';
const PROJECT_URL = /^\/project\/(\d+)(?:[/?#]|$)/;

/**
 * The project the side menu works with: the one in the URL on project pages, otherwise the last
 * visited one, so global pages (Správy, Agent chat, admin) still have a project to link to.
 */
@Injectable({ providedIn: 'root' })
export class ProjectContextStore {
    private readonly router = inject(Router);
    private readonly projectApi = inject(ProjectApi);
    private readonly projectStore = inject(ProjectStore);

    private readonly _projects = signal<Project[]>([]);
    private readonly url = signal(this.router.url);
    private readonly idLastProject = signal<number | null>(ProjectContextStore.readLastId());

    public readonly projects = this._projects.asReadonly();

    private readonly idRouteProject = computed(() => ProjectContextStore.idFromUrl(this.url()));

    public readonly isOnProjectRoute = computed(() => this.idRouteProject() !== null);

    public readonly idProject = computed<number | null>(
        () =>
            this.idRouteProject() ?? this.idLastProject() ?? this._projects()[0]?.idProject ?? null
    );

    public readonly project = computed<Project | null>(
        () => this._projects().find(p => p.idProject === this.idProject()) ?? null
    );

    public constructor() {
        this.router.events
            .pipe(
                filter((e): e is NavigationEnd => e instanceof NavigationEnd),
                takeUntilDestroyed()
            )
            .subscribe(e => this.onNavigation(e.urlAfterRedirects));
        // Keeps renamed or freshly created projects in the list without a reload.
        this.projectStore.project$
            .pipe(takeUntilDestroyed())
            .subscribe(project => this.upsert(project));
    }

    public load(): void {
        this.projectApi.load$().subscribe(projects => {
            this._projects.set(projects);
            this.forgetUnknownLastProject(projects);
            this.redirectToLastProject();
        });
    }

    public upsert(project: Project): void {
        this._projects.update(current =>
            orderBy(
                [...current.filter(p => p.idProject !== project.idProject), project],
                ['name'],
                ['asc']
            )
        );
    }

    private onNavigation(url: string): void {
        this.url.set(url);
        const idProject = ProjectContextStore.idFromUrl(url);
        if (idProject !== null && idProject !== this.idLastProject()) {
            this.idLastProject.set(idProject);
            ProjectContextStore.writeLastId(idProject);
        }
    }

    private forgetUnknownLastProject(projects: Project[]): void {
        const idLast = this.idLastProject();
        if (idLast !== null && !projects.some(p => p.idProject === idLast)) {
            this.idLastProject.set(null);
            ProjectContextStore.writeLastId(null);
        }
    }

    private redirectToLastProject(): void {
        const idLast = this.idLastProject();
        if (this.router.url === '/' && idLast !== null) {
            void this.router.navigate(['/project', idLast, 'view']);
        }
    }

    private static idFromUrl(url: string): number | null {
        const match = PROJECT_URL.exec(url);
        return match ? Number(match[1]) : null;
    }

    private static readLastId(): number | null {
        try {
            const id = Number(localStorage.getItem(LAST_PROJECT_KEY));
            return Number.isInteger(id) && id > 0 ? id : null;
        } catch {
            return null;
        }
    }

    private static writeLastId(idProject: number | null): void {
        try {
            if (idProject === null) {
                localStorage.removeItem(LAST_PROJECT_KEY);
            } else {
                localStorage.setItem(LAST_PROJECT_KEY, String(idProject));
            }
        } catch {
            // Storage blocked (private mode): the context simply is not remembered.
        }
    }
}

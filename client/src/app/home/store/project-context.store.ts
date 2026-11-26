import { Injectable, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed, toObservable, toSignal } from '@angular/core/rxjs-interop';
import { NavigationEnd, Router } from '@angular/router';
import orderBy from 'lodash-es/orderBy';
import { catchError, filter, map, of, switchMap } from 'rxjs';
import { ProjectMemberApi } from 'src/app/project/api/project-member.api.service';
import { ProjectApi } from 'src/app/project/api/project.api.service';
import { Project } from 'src/app/project/model/project.model';
import { ProjectStore } from 'src/app/project/project.store';
import { ROLE_RANK, Role } from 'src/app/shared/constants/role.enum';

const LAST_PROJECT_KEY = 'lastProjectId';
const RECENT_PROJECTS_KEY = 'recentProjectIds';
const MAX_RECENT = 3;
const PROJECT_URL = /^\/project\/(\d+)(?:[/?#]|$)/;

/**
 * The project the side menu works with: the one in the URL on project pages, otherwise the last
 * visited one, so global pages (Správy, Agent chat, admin) still have a project to link to.
 */
@Injectable({ providedIn: 'root' })
export class ProjectContextStore {
    private readonly router = inject(Router);
    private readonly projectApi = inject(ProjectApi);
    private readonly memberApi = inject(ProjectMemberApi);
    private readonly projectStore = inject(ProjectStore);

    private readonly _projects = signal<Project[]>([]);
    private readonly url = signal(this.router.url);
    private readonly idLastProject = signal<number | null>(ProjectContextStore.readLastId());
    private readonly idsRecent = signal<number[]>(
        ProjectContextStore.readRecentIds(this.idLastProject())
    );

    public readonly projects = this._projects.asReadonly();

    /** Most recently visited projects first, for the project picker. */
    public readonly recent = computed<Project[]>(() =>
        this.idsRecent()
            .map(id => this._projects().find(p => p.idProject === id))
            .filter((p): p is Project => p !== undefined)
    );

    private readonly idRouteProject = computed(() => ProjectContextStore.idFromUrl(this.url()));

    public readonly isOnProjectRoute = computed(() => this.idRouteProject() !== null);

    public readonly idProject = computed<number | null>(
        () =>
            this.idRouteProject() ?? this.idLastProject() ?? this._projects()[0]?.idProject ?? null
    );

    public readonly project = computed<Project | null>(
        () => this._projects().find(p => p.idProject === this.idProject()) ?? null
    );

    /** Loaded here, because AclStore knows the role only while a project page is open. */
    private readonly role = toSignal(
        toObservable(this.idProject).pipe(
            switchMap(idProject =>
                idProject === null
                    ? of(null)
                    : this.memberApi.loadUserRole$(idProject).pipe(
                          map(res => res.role),
                          catchError(() => of(null))
                      )
            )
        ),
        { initialValue: null }
    );

    public readonly canUpdateProject = computed(() => {
        const role = this.role();
        return role !== null && ROLE_RANK[role] >= ROLE_RANK[Role.Owner];
    });

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
        if (idProject !== null && idProject !== this.idsRecent()[0]) {
            const ids = [idProject, ...this.idsRecent().filter(id => id !== idProject)];
            this.idsRecent.set(ids.slice(0, MAX_RECENT));
            ProjectContextStore.writeRecentIds(this.idsRecent());
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

    /** Falls back to the last project so the first picker after this release is not empty. */
    private static readRecentIds(idLast: number | null): number[] {
        try {
            const ids: unknown = JSON.parse(localStorage.getItem(RECENT_PROJECTS_KEY) ?? 'null');
            if (Array.isArray(ids)) {
                return ids
                    .filter((id: unknown): id is number => Number.isInteger(id) && Number(id) > 0)
                    .slice(0, MAX_RECENT);
            }
        } catch {
            // Blocked storage or a damaged value: start over.
        }
        return idLast === null ? [] : [idLast];
    }

    private static writeRecentIds(ids: number[]): void {
        try {
            localStorage.setItem(RECENT_PROJECTS_KEY, JSON.stringify(ids));
        } catch {
            // Storage blocked (private mode): the list lives until reload.
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

import {
    ChangeDetectionStrategy,
    Component,
    computed,
    inject,
    OnInit,
    signal
} from '@angular/core';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { UiMenuItem } from 'src/app/ui/components/menu/menu-item.model';

interface AppMenuItem extends UiMenuItem {
    tablerIcon?: string;
}
import { AuthStore } from 'src/app/auth/store/auth.store';
import { Project } from 'src/app/project/model/project.model';
import { ProjectFormWindowComponent } from 'src/app/project/components/project-form-window/project-form-window.component';
import { ProjectService } from 'src/app/project/project.service';
import { WindowService } from 'src/app/shared/window/window.service';
import orderBy from 'lodash-es/orderBy';
import { ProjectStore } from 'src/app/project/project.store';
import { NavigationEnd, Router } from '@angular/router';
import { toSignal } from '@angular/core/rxjs-interop';
import { filter, map, startWith } from 'rxjs';

@Component({
    selector: 'app-top-menu',
    templateUrl: './top-menu.component.html',
    styleUrls: ['./top-menu.component.scss'],
    providers: [WindowService],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class TopMenuComponent implements OnInit {
    private readonly router = inject(Router);
    private readonly i18n = inject(I18nService);
    private readonly authStore = inject(AuthStore);
    private readonly sProject = inject(ProjectService);
    private readonly projectStore = inject(ProjectStore);
    private readonly sWindow = inject(WindowService);

    protected readonly projects = signal<Project[]>([]);
    protected readonly project = toSignal(this.projectStore.project$);

    private readonly currentUrl = toSignal(
        this.router.events.pipe(
            filter(e => e instanceof NavigationEnd),
            map(() => this.router.url),
            startWith(this.router.url)
        )
    );

    protected readonly idProjectSelected = computed(() => {
        if (!this.currentUrl()?.startsWith('/project')) return null;
        return this.project()?.idProject ?? null;
    });
    protected readonly user = this.authStore.user;

    protected readonly userMenuItems = computed<AppMenuItem[]>(() => {
        const items: AppMenuItem[] = [
            {
                label: this.i18n.instant('USER.MY_PAGE'),
                tablerIcon: 'user',
                routerLink: ['/user']
            },
            {
                label: this.i18n.instant('USER.SETTINGS'),
                tablerIcon: 'settings',
                routerLink: ['/user/settings']
            },
            {
                label: this.i18n.instant('LOGOUT'),
                tablerIcon: 'logout',
                routerLink: ['/logout']
            }
        ];
        if (this.user()?.isAdmin) {
            items.push(
                { separator: true },
                {
                    label: this.i18n.instant('ADMIN.USERS'),
                    tablerIcon: 'users',
                    routerLink: ['/admin/users']
                },
                {
                    label: this.i18n.instant('SKILL.TITLE'),
                    tablerIcon: 'sparkles',
                    routerLink: ['/admin/skills']
                },
                {
                    label: this.i18n.instant('ADMIN.SETTINGS'),
                    tablerIcon: 'settings',
                    routerLink: ['/admin/settings']
                }
            );
        }
        return items;
    });

    private readonly LAST_PROJECT_KEY = 'lastProjectId';

    public ngOnInit(): void {
        this.sProject.loadProjects().subscribe(projects => {
            this.projects.set(projects);
            this.redirectToLastProject(projects);
        });
    }

    protected onProjectSelect(value: unknown): void {
        const idProject = Number(value);
        localStorage.setItem(this.LAST_PROJECT_KEY, String(idProject));
        void this.router.navigate(['/project', idProject, 'view']);
    }

    protected onNewProject(): void {
        this.sWindow
            .open<Project | null>(ProjectFormWindowComponent, {
                header: this.i18n.instant('PROJECT.NEW'),
                data: { project: { idProject: 0, name: '', color: '' } }
            })
            .onClose.subscribe(project => {
                if (project) {
                    this.projects.update(current =>
                        orderBy([...current, project], ['name'], ['asc'])
                    );
                }
            });
    }

    private redirectToLastProject(projects: Project[]): void {
        if (this.router.url !== '/') {
            return;
        }
        const saved = localStorage.getItem(this.LAST_PROJECT_KEY);
        if (!saved) {
            return;
        }
        const idProject = Number(saved);
        const exists = projects.some(p => p.idProject === idProject);
        if (exists) {
            void this.router.navigate(['/project', idProject, 'view']);
        } else {
            localStorage.removeItem(this.LAST_PROJECT_KEY);
        }
    }
}

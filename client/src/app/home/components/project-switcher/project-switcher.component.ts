import {
    ChangeDetectionStrategy,
    Component,
    ElementRef,
    OnDestroy,
    OnInit,
    computed,
    inject,
    signal,
    viewChild
} from '@angular/core';
import { Router } from '@angular/router';
import { FuzzySearch } from 'src/app/core/command/fuzzy-search';
import { HotkeyService } from 'src/app/core/command/hotkey.service';
import { ProjectFormWindowComponent } from 'src/app/project/components/project-form-window/project-form-window.component';
import { Project } from 'src/app/project/model/project.model';
import { Abbreviation } from 'src/app/shared/abbreviation/abbreviation';
import { Color } from 'src/app/shared/color/color';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { WindowService } from 'src/app/shared/window/window.service';
import { UiPopoverComponent } from 'src/app/ui/components/popover/popover.component';
import { ProjectContextStore } from '../../store/project-context.store';
import { ProjectSwitchTarget } from './project-switch-target';

interface ProjectGroup {
    headingKey: string;
    projects: Project[];
}

/** Project picker in the side menu header: recent projects, all projects, search, new project. */
@Component({
    selector: 'app-project-switcher',
    templateUrl: './project-switcher.component.html',
    styleUrls: ['./project-switcher.component.scss'],
    providers: [WindowService],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ProjectSwitcherComponent implements OnInit, OnDestroy {
    private static nextId = 0;

    private readonly router = inject(Router);
    private readonly i18n = inject(I18nService);
    private readonly hotkeys = inject(HotkeyService);
    private readonly projectContext = inject(ProjectContextStore);
    private readonly sWindow = inject(WindowService);

    private readonly trigger = viewChild.required<ElementRef<HTMLButtonElement>>('trigger');
    private readonly popover = viewChild.required<UiPopoverComponent>('popover');
    private readonly search = viewChild.required<ElementRef<HTMLInputElement>>('search');

    protected readonly baseId = `project-switcher-${ProjectSwitcherComponent.nextId++}`;

    protected readonly project = this.projectContext.project;
    protected readonly isOpen = signal(false);
    protected readonly query = signal('');
    protected readonly activeIndex = signal(0);

    protected readonly groups = computed<ProjectGroup[]>(() =>
        this.buildGroups().filter(group => group.projects.length > 0)
    );

    protected readonly options = computed(() => this.groups().flatMap(group => group.projects));

    protected readonly activeProject = computed(() => this.options()[this.activeIndex()] ?? null);

    protected readonly activeOptionId = computed(() => {
        const project = this.activeProject();
        return project ? this.optionId(project) : null;
    });

    public ngOnInit(): void {
        this.hotkeys.registerProjectPicker({ open: () => this.open() });
    }

    public ngOnDestroy(): void {
        this.hotkeys.registerProjectPicker(null);
    }

    public open(): void {
        if (this.isOpen()) {
            return;
        }
        this.query.set('');
        this.activeIndex.set(0);
        this.popover().show(this.trigger().nativeElement);
        this.isOpen.set(true);
        this.search().nativeElement.focus();
    }

    protected onTriggerClick(): void {
        if (this.isOpen()) {
            this.popover().hide();
        } else {
            this.open();
        }
    }

    protected onClosed(): void {
        this.isOpen.set(false);
        // Escape or a pick removes the focused search box; keep the keyboard on the trigger.
        if (!document.activeElement || document.activeElement === document.body) {
            this.trigger().nativeElement.focus();
        }
    }

    protected onQuery(event: Event): void {
        this.query.set((event.target as HTMLInputElement).value);
        this.activeIndex.set(0);
    }

    protected onKeydown(event: KeyboardEvent): void {
        const count = this.options().length;
        switch (event.key) {
            case 'ArrowDown':
                event.preventDefault();
                this.activeIndex.set(count ? (this.activeIndex() + 1) % count : 0);
                break;
            case 'ArrowUp':
                event.preventDefault();
                this.activeIndex.set(count ? (this.activeIndex() - 1 + count) % count : 0);
                break;
            case 'Enter': {
                event.preventDefault();
                const project = this.activeProject();
                if (project) {
                    this.onPick(project);
                }
                break;
            }
        }
    }

    protected onPointerMove(project: Project): void {
        this.activeIndex.set(this.options().indexOf(project));
    }

    protected onPick(project: Project): void {
        this.popover().hide();
        const isCurrent =
            this.projectContext.isOnProjectRoute() &&
            project.idProject === this.projectContext.idProject();
        if (!isCurrent) {
            void this.router.navigate(ProjectSwitchTarget.url(this.router.url, project.idProject));
        }
    }

    protected onNewProject(): void {
        this.popover().hide();
        this.sWindow
            .open<Project | null>(ProjectFormWindowComponent, {
                header: this.i18n.instant('PROJECT.NEW'),
                data: { project: { idProject: 0, name: '', color: '' } }
            })
            .onClose.subscribe(project => {
                if (project) {
                    this.projectContext.upsert(project);
                }
            });
    }

    private buildGroups(): ProjectGroup[] {
        const projects = this.projectContext.projects();
        const query = this.query().trim();
        if (query) {
            const found = projects
                .map(project => ({ project, ...FuzzySearch.match(query, project.name) }))
                .filter(hit => hit.matched)
                .sort((a, b) => b.score - a.score)
                .map(hit => hit.project);
            return [{ headingKey: 'PROJECT.SWITCHER.RESULTS', projects: found }];
        }
        const recent = this.projectContext.recent();
        return [
            { headingKey: 'PROJECT.SWITCHER.RECENT', projects: recent },
            {
                headingKey: 'PROJECT.SWITCHER.ALL',
                projects: projects.filter(p => !recent.includes(p))
            }
        ];
    }

    protected isCurrent(project: Project): boolean {
        return project.idProject === this.projectContext.idProject();
    }

    protected optionId(project: Project): string {
        return `${this.baseId}-option-${project.idProject}`;
    }

    protected abbreviation(project: Project): string {
        return Abbreviation.of(project.name);
    }

    protected tileColor(project: Project): string {
        return project.color ? Color.getContrastColor(project.color) : '';
    }
}

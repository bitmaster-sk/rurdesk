import { Component, computed, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { TranslateLoader, TranslateModule } from '@ngx-translate/core';
import { Observable, Subject, of } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { HotkeyService, ProjectPickerHandler } from 'src/app/core/command/hotkey.service';
import { Project } from 'src/app/project/model/project.model';
import { WindowService } from 'src/app/shared/window/window.service';
import { UiDividerComponent } from 'src/app/ui/components/divider/divider.component';
import { UiPopoverComponent } from 'src/app/ui/components/popover/popover.component';
import { TablerIconStub } from 'src/testing/stubs';
import { ProjectContextStore } from '../../store/project-context.store';
import { ProjectSwitcherComponent } from './project-switcher.component';

@Component({ selector: 'app-page-stub', template: '', standalone: true })
class PageStub {}

class EmptyLoader implements TranslateLoader {
    public getTranslation(): Observable<Record<string, string>> {
        return of({});
    }
}

const project = (idProject: number, name: string): Project => ({
    idProject,
    name,
    color: '#1a3263'
});

const PROJECTS = [
    project(1, 'Alpha'),
    project(2, 'Beta'),
    project(3, 'Gamma'),
    project(4, 'Delta')
];

interface Setup {
    element: HTMLElement;
    router: Router;
    upsert: ReturnType<typeof vi.fn>;
    picker: () => ProjectPickerHandler | null;
    windowClose$: Subject<Project | null>;
}

async function setup(url: string, idProject = 1, idsRecent = [1, 3]): Promise<Setup> {
    const projects = signal<Project[]>(PROJECTS);
    const idRoute = signal(idProject);
    const context = {
        projects,
        recent: computed(() => idsRecent.map(id => projects().find(p => p.idProject === id)!)),
        idProject: idRoute,
        isOnProjectRoute: signal(url.startsWith('/project')),
        project: computed(() => projects().find(p => p.idProject === idRoute()) ?? null),
        upsert: vi.fn((p: Project) => projects.set([...projects(), p]))
    };
    let picker: ProjectPickerHandler | null = null;
    const windowClose$ = new Subject<Project | null>();

    await TestBed.configureTestingModule({
        declarations: [ProjectSwitcherComponent, UiPopoverComponent, UiDividerComponent],
        imports: [
            TranslateModule.forRoot({
                loader: { provide: TranslateLoader, useClass: EmptyLoader }
            }),
            TablerIconStub
        ],
        providers: [
            provideRouter([{ path: '**', component: PageStub }]),
            { provide: ProjectContextStore, useValue: context },
            {
                provide: HotkeyService,
                useValue: {
                    registerProjectPicker: (handler: ProjectPickerHandler | null) =>
                        (picker = handler)
                }
            }
        ]
    })
        .overrideComponent(ProjectSwitcherComponent, {
            set: {
                providers: [
                    {
                        provide: WindowService,
                        useValue: { open: () => ({ onClose: windowClose$.asObservable() }) }
                    }
                ]
            }
        })
        .compileComponents();
    const router = TestBed.inject(Router);
    await router.navigateByUrl(url);
    const fixture = TestBed.createComponent(ProjectSwitcherComponent);
    fixture.autoDetectChanges();
    await fixture.whenStable();
    return {
        element: fixture.nativeElement,
        router,
        upsert: context.upsert,
        picker: () => picker,
        windowClose$
    };
}

const trigger = (element: HTMLElement): HTMLButtonElement =>
    element.querySelector<HTMLButtonElement>('[data-testid="project-switcher"]')!;

const search = (): HTMLInputElement =>
    document.querySelector<HTMLInputElement>('[data-testid="project-switcher-search"]')!;

const optionNames = (): string[] =>
    [...document.querySelectorAll('[data-testid="project-switcher-option"]')].map(
        o => o.textContent?.trim().replace(/^\S+\s+/, '') ?? ''
    );

const headings = (): string[] =>
    [...document.querySelectorAll('.picker__heading')].map(h => h.textContent?.trim() ?? '');

function type(value: string): void {
    search().value = value;
    search().dispatchEvent(new Event('input'));
}

function press(key: string): void {
    search().dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true }));
}

describe('ProjectSwitcherComponent', () => {
    it('shows the current project with its two-letter tile', async () => {
        const { element } = await setup('/project/2/view', 2);

        expect(trigger(element).textContent).toContain('Beta');
        expect(trigger(element).querySelector('.tile')?.textContent?.trim()).toBe('BE');
    });

    it('lists recent projects first, then the rest, and marks the current one', async () => {
        const { element } = await setup('/project/1/view');

        trigger(element).click();

        await vi.waitFor(() => expect(optionNames()).toEqual(['Alpha', 'Gamma', 'Beta', 'Delta']));
        expect(headings()).toEqual(['PROJECT.SWITCHER.RECENT', 'PROJECT.SWITCHER.ALL']);
        const current = document.querySelectorAll('[aria-current="true"]');
        expect(current).toHaveLength(1);
        expect(current[0].textContent).toContain('Alpha');
        expect(document.activeElement).toBe(search());
    });

    it('searches across both groups', async () => {
        const { element } = await setup('/project/1/view');
        trigger(element).click();

        type('ta');

        await vi.waitFor(() => expect(optionNames()).toEqual(['Beta', 'Delta']));
        expect(headings()).toEqual(['PROJECT.SWITCHER.RESULTS']);
    });

    it('says so when nothing matches', async () => {
        const { element } = await setup('/project/1/view');
        trigger(element).click();

        type('zzz');

        await vi.waitFor(() =>
            expect(document.querySelector('.picker__empty')?.textContent).toContain(
                'PROJECT.SWITCHER.NO_RESULTS'
            )
        );
    });

    it('switches with the keyboard and keeps the issue view', async () => {
        const { element, router } = await setup('/project/1/issue/view/kanban');
        trigger(element).click();

        press('ArrowDown');
        press('Enter');

        await vi.waitFor(() => expect(router.url).toBe('/project/3/issue/view/kanban'));
        expect(document.querySelector('[data-testid="project-switcher-search"]')).toBeNull();
        expect(document.activeElement).toBe(trigger(element));
    });

    it('opens the project overview from an issue detail', async () => {
        const { element, router } = await setup('/project/1/issue/123');
        trigger(element).click();

        type('delta');
        press('Enter');

        await vi.waitFor(() => expect(router.url).toBe('/project/4/view'));
    });

    it('opens the overview of the side menu project from a page outside the project', async () => {
        const { element, router } = await setup('/message/1/team/view');
        trigger(element).click();

        press('Enter');

        await vi.waitFor(() => expect(router.url).toBe('/project/1/view'));
    });

    it('closes on Escape and returns focus to the trigger', async () => {
        const { element } = await setup('/project/1/view');
        trigger(element).click();

        press('Escape');

        await vi.waitFor(() =>
            expect(trigger(element).getAttribute('aria-expanded')).toBe('false')
        );
        expect(document.activeElement).toBe(trigger(element));
    });

    it('opens from the p shortcut', async () => {
        const { element, picker } = await setup('/project/1/view');

        picker()?.open();

        await vi.waitFor(() => expect(trigger(element).getAttribute('aria-expanded')).toBe('true'));
        expect(document.activeElement).toBe(search());
    });

    it('adds a newly created project to the list', async () => {
        const { element, upsert, windowClose$ } = await setup('/project/1/view');
        trigger(element).click();

        document.querySelector<HTMLElement>('[data-testid="project-switcher-new"]')?.click();
        windowClose$.next(project(5, 'Epsilon'));

        expect(upsert).toHaveBeenCalledWith(project(5, 'Epsilon'));
        trigger(element).click();
        await vi.waitFor(() => expect(optionNames()).toContain('Epsilon'));
    });
});

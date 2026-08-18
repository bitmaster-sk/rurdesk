import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { Subject, of } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { UserService } from 'src/app/auth/user.service';
import { SavedViewApi } from 'src/app/project/api/saved-view.api.service';
import { SavedView } from 'src/app/project/model/saved-view.model';
import { AclStore } from 'src/app/project/store/acl.store';
import { SavedViewStore } from 'src/app/project/store/saved-view.store';
import { UiModule } from 'src/app/ui/ui.module';
import { UiToastService } from 'src/app/ui/service/ui-toast.service';
import { TablerIconStub } from 'src/testing/stubs';
import { IssueViewMode } from '../../constants/issue-view-modes.enum';
import { SavedViewApplyService } from '../../service/saved-view-apply.service';
import { IssueFilterStore } from '../filter/issue-filter.store';
import { SavedViewMenuComponent } from './saved-view-menu.component';

const ME = 1;
const SOMEONE_ELSE = 2;
const ID_PROJECT = 5;

function view(over: Partial<SavedView> = {}): SavedView {
    return {
        idSavedView: 1,
        idProject: ID_PROJECT,
        name: 'My bugs',
        viewType: IssueViewMode.TABLE,
        isShared: false,
        createBy: ME,
        updateAt: '2026-08-01T00:00:00Z',
        config: { v: 1 },
        ...over
    };
}

@Component({ selector: 'app-saved-view-dialog', template: '', standalone: true })
class SavedViewDialogStub {}

describe('SavedViewMenuComponent (browser)', () => {
    let apply: ReturnType<typeof vi.fn>;
    let markUrl: ReturnType<typeof vi.fn>;
    let api: Record<'loadByProject$' | 'create$' | 'edit$' | 'delete$', ReturnType<typeof vi.fn>>;
    let store: SavedViewStore;
    let canUpdateProject: ReturnType<typeof signal<boolean>>;
    let canCreateIssue: ReturnType<typeof signal<boolean>>;
    let isFilterEdited: Subject<void>;

    beforeEach(async () => {
        canUpdateProject = signal(false);
        canCreateIssue = signal(true);
        isFilterEdited = new Subject<void>();
        apply = vi.fn();
        markUrl = vi.fn();
        api = {
            loadByProject$: vi.fn(() => of<SavedView[]>([])),
            create$: vi.fn(() => of(view({ idSavedView: 42 }))),
            edit$: vi.fn(() => of(view())),
            delete$: vi.fn(() => of(undefined))
        };

        await TestBed.configureTestingModule({
            imports: [
                TranslateModule.forRoot(),
                FormsModule,
                UiModule,
                TablerIconStub,
                SavedViewDialogStub
            ],
            declarations: [SavedViewMenuComponent],
            providers: [
                { provide: SavedViewApi, useValue: api },
                {
                    provide: SavedViewApplyService,
                    useValue: { apply, markUrl, currentMode: () => IssueViewMode.KANBAN }
                },
                {
                    provide: IssueFilterStore,
                    useValue: {
                        isFilterEdited$: isFilterEdited.asObservable(),
                        getFilter: () => ({
                            idProject: ID_PROJECT,
                            idsState: [3],
                            orderColumn: 'title',
                            orderDirection: 'asc' as const
                        })
                    }
                },
                { provide: AclStore, useValue: { canUpdateProject, canCreateIssue } },
                { provide: UiToastService, useValue: { show: vi.fn() } },
                { provide: UserService, useValue: { user: { getValue: () => ({ idUser: ME }) } } },
                { provide: Router, useValue: { navigate: vi.fn(), serializeUrl: () => '/x' } }
            ]
        }).compileComponents();

        store = TestBed.inject(SavedViewStore);
    });

    afterEach(() => {
        document.querySelectorAll('.cdk-overlay-container').forEach(node => node.remove());
    });

    interface Page {
        fixture: ComponentFixture<SavedViewMenuComponent>;
        component: Record<string, any>;
        open: () => void;
        panel: () => HTMLElement | null;
        trigger: () => HTMLElement;
    }

    function render(loaded: SavedView[] = []): Page {
        api.loadByProject$.mockReturnValue(of(loaded));
        const fixture = TestBed.createComponent(SavedViewMenuComponent);
        fixture.componentRef.setInput('idProject', ID_PROJECT);
        store.load(ID_PROJECT);
        fixture.detectChanges();
        const trigger = (): HTMLElement =>
            fixture.nativeElement.querySelector('.ui-button-group button');
        return {
            fixture,
            component: fixture.componentInstance,
            trigger,
            panel: () => document.querySelector<HTMLElement>('.sv-panel'),
            open: () => {
                trigger().click();
                fixture.detectChanges();
            }
        };
    }

    // ── behaviour ───────────────────────────────────────────────────────────

    it('delegates apply to the service, which owns the view-type decision', () => {
        const target = view();
        const page = render([target]);

        page.component.onApply(target);

        expect(apply).toHaveBeenCalledWith(target, ID_PROJECT);
    });

    it('splits shared and private views, preserving store order', () => {
        const page = render([
            view({ idSavedView: 1, name: 'team', isShared: true }),
            view({ idSavedView: 2, name: 'alpha' }),
            view({ idSavedView: 3, name: 'zebra' })
        ]);

        expect(page.component.sharedViews().map((item: SavedView) => item.name)).toEqual(['team']);
        expect(page.component.privateViews().map((item: SavedView) => item.name)).toEqual([
            'alpha',
            'zebra'
        ]);
    });

    it('filters both groups case-insensitively by name', () => {
        const page = render([
            view({ idSavedView: 1, name: 'Team Bugs', isShared: true }),
            view({ idSavedView: 2, name: 'my bugs' }),
            view({ idSavedView: 3, name: 'Roadmap' })
        ]);

        page.component.query.set('BUG');

        expect(page.component.matching().map((item: SavedView) => item.idSavedView)).toEqual([
            1, 2
        ]);
    });

    it('Enter applies the view when exactly one match is left', () => {
        const target = view({ idSavedView: 2, name: 'unique' });
        const page = render([view({ idSavedView: 1, name: 'other' }), target]);
        page.component.query.set('uniq');

        page.component.onSearchEnter(new Event('keydown'));

        expect(apply).toHaveBeenCalledWith(target, ID_PROJECT);
    });

    it.each([
        ['zero', 'nothing', 0],
        ['several', 'bug', 2]
    ])('Enter does nothing with %s matches', (_label, query, expected) => {
        const page = render([
            view({ idSavedView: 1, name: 'a bug' }),
            view({ idSavedView: 2, name: 'b bug' })
        ]);
        page.component.query.set(query);
        expect(page.component.matching()).toHaveLength(expected);

        page.component.onSearchEnter(new Event('keydown'));

        expect(apply).not.toHaveBeenCalled();
    });

    it('saving a new view sends the live filter and the current view type', () => {
        const page = render();

        page.component.onSaveCurrent();
        page.component.onDialogSaved({ name: 'Fresh', isShared: true });

        expect(api.create$).toHaveBeenCalledWith(ID_PROJECT, {
            name: 'Fresh',
            viewType: IssueViewMode.KANBAN,
            config: { v: 1, idsState: [3], orderColumn: 'title', orderDirection: 'asc' },
            isShared: true
        });
    });

    it('saving a kanban view records the board layout that is on screen', () => {
        const page = render();
        store.setLiveKanbanLayout('swimlane');

        page.component.onSaveCurrent();
        page.component.onDialogSaved({ name: 'Board', isShared: false });

        expect(api.create$.mock.calls[0][1].config.kanbanLayout).toBe('swimlane');
    });

    it('marks a newly created view as applied and puts it in the URL', () => {
        const page = render();

        page.component.onSaveCurrent();
        page.component.onDialogSaved({ name: 'Fresh', isShared: false });

        expect(store.idAppliedView()).toBe(42);
        expect(markUrl).toHaveBeenCalledWith(42);
    });

    it('renaming keeps the stored config', () => {
        const target = view({ config: { v: 1, idsState: [99] } });
        const page = render([target]);

        page.component.onRename(target);
        page.component.onDialogSaved({ name: 'Renamed', isShared: false });

        expect(api.edit$).toHaveBeenCalledWith(1, {
            name: 'Renamed',
            viewType: IssueViewMode.TABLE,
            config: { v: 1, idsState: [99] },
            isShared: false
        });
    });

    it('toggling shared keeps the stored config and the name', () => {
        const target = view({ config: { v: 1, idsState: [99] } });
        const page = render([target]);

        page.component.onToggleShared(target);

        expect(api.edit$).toHaveBeenCalledWith(1, {
            name: 'My bugs',
            viewType: IssueViewMode.TABLE,
            config: { v: 1, idsState: [99] },
            isShared: true
        });
    });

    it('updating the active view overwrites its config with the live filter', () => {
        const target = view({ config: { v: 1, idsState: [99] } });
        const page = render([target]);
        store.setApplied(1);
        store.setLiveKanbanLayout('columns');

        page.component.onUpdateActive();

        expect(api.edit$).toHaveBeenCalledWith(1, {
            name: 'My bugs',
            viewType: IssueViewMode.TABLE,
            config: {
                v: 1,
                idsState: [3],
                orderColumn: 'title',
                orderDirection: 'asc',
                kanbanLayout: 'columns'
            },
            isShared: false
        });
    });

    it('update is unavailable with no active view', () => {
        const page = render([view()]);

        expect(page.component.canManageActive()).toBe(false);
        page.component.onUpdateActive();
        expect(api.edit$).not.toHaveBeenCalled();
    });

    it('deleting reloads the list and drops the highlight and the URL param', () => {
        const target = view();
        const page = render([target]);
        store.setApplied(1);
        api.loadByProject$.mockClear();

        page.component.onDelete(target);

        expect(api.delete$).toHaveBeenCalledWith(1);
        expect(store.idAppliedView()).toBeNull();
        // A stale id would make the reloaded list report the view as missing.
        expect(markUrl).toHaveBeenCalledWith(null);
        expect(api.loadByProject$).toHaveBeenCalledWith(ID_PROJECT);
    });

    it('deleting a view that is not applied leaves the URL alone', () => {
        const target = view({ idSavedView: 9 });
        const page = render([view(), target]);
        store.setApplied(1);

        page.component.onDelete(target);

        expect(store.idAppliedView()).toBe(1);
        expect(markUrl).not.toHaveBeenCalled();
    });

    it('hides edit affordances on someone else’s view for a non-owner', () => {
        const foreign = view({ createBy: SOMEONE_ELSE });
        const page = render([foreign]);

        expect(page.component.canManage(foreign)).toBe(false);
    });

    it('a project owner may manage any view', () => {
        const foreign = view({ createBy: SOMEONE_ELSE });
        const page = render([foreign]);
        canUpdateProject.set(true);

        expect(page.component.canManage(foreign)).toBe(true);
    });

    it('clearing drops the highlight, the URL param and the view’s filter', () => {
        const page = render([view()]);
        store.setApplied(1);
        const filterReset = vi.fn();
        store.filterResetSignal$.subscribe(filterReset);

        page.component.onClearActive();

        expect(store.idAppliedView()).toBeNull();
        expect(markUrl).toHaveBeenCalledWith(null);
        expect(filterReset).toHaveBeenCalledTimes(1);
        expect(apply).not.toHaveBeenCalled();
    });

    it('keeps following the view when the filter is edited, flagging it instead', () => {
        render([view()]);
        store.setApplied(1);

        isFilterEdited.next();

        expect(store.idAppliedView()).toBe(1);
        expect(store.isUnsaved()).toBe(true);
        expect(markUrl).not.toHaveBeenCalled();
    });

    it('a filter edit with no view applied flags nothing', () => {
        render([view()]);

        isFilterEdited.next();

        expect(store.isUnsaved()).toBe(false);
    });

    it('updating the view clears the unsaved flag', () => {
        const page = render([view()]);
        store.setApplied(1);
        isFilterEdited.next();

        page.component.onUpdateActive();

        expect(store.isUnsaved()).toBe(false);
    });

    it('re-applying the view discards the unsaved changes', () => {
        const target = view();
        const page = render([target]);
        store.setApplied(1);
        isFilterEdited.next();

        // apply() is stubbed here, so drive the state the way the real one does
        page.component.onApply(target);
        store.setApplied(1);

        expect(store.isUnsaved()).toBe(false);
    });

    it('the trigger shows an unsaved-changes marker', () => {
        const page = render([view()]);
        store.setApplied(1);
        page.fixture.detectChanges();
        expect(page.fixture.nativeElement.querySelector('.sv-changed')).toBeNull();

        isFilterEdited.next();
        page.fixture.detectChanges();

        expect(page.fixture.nativeElement.querySelector('.sv-changed')).not.toBeNull();
    });

    // ── rendered panel ──────────────────────────────────────────────────────

    it('renders each row as a container with sibling action buttons', () => {
        const page = render([view()]);
        page.open();

        const row = page.panel()!.querySelector('.sv-item')!;
        expect(row.tagName).toBe('DIV');
        expect(row.querySelector('.sv-apply button')).toBeNull();
        expect(row.querySelectorAll('.sv-actions .sv-act').length).toBeGreaterThan(0);
    });

    // Focusing the row's apply button triggers :focus-within, which un-hides the strip so
    // Tab can reach it. Under display:none it would be unfocusable, i.e. mouse-only.
    it('reveals the row actions so the keyboard can reach them', () => {
        const page = render([view()]);
        page.open();
        const row = page.panel()!.querySelector<HTMLElement>('.sv-item')!;
        const strip = row.querySelector<HTMLElement>('.sv-actions')!;
        const action = row.querySelector<HTMLButtonElement>('.sv-act')!;
        expect(getComputedStyle(strip).visibility).toBe('hidden');

        row.querySelector<HTMLButtonElement>('.sv-apply')!.focus();

        expect(getComputedStyle(strip).visibility).toBe('visible');
        action.focus();
        expect(document.activeElement).toBe(action);
    });

    it('marks the applied row for assistive tech, not only visually', () => {
        const page = render([view()]);
        store.setApplied(1);
        page.open();

        expect(
            page.panel()!.querySelector('.sv-item.active .sv-apply')!.getAttribute('aria-current')
        ).toBe('true');
    });

    it('keeps the search input rendered in the empty and no-match states', () => {
        const page = render();
        page.open();
        expect(page.panel()!.querySelector('.sv-search input')).not.toBeNull();
        expect(page.panel()!.querySelector('.sv-empty')).not.toBeNull();

        page.component.query.set('nothing');
        page.fixture.detectChanges();

        expect(page.panel()!.querySelector('.sv-search input')).not.toBeNull();
    });

    it('names the panel so the trigger’s aria-haspopup is not a lie', () => {
        const page = render([view()]);
        page.open();

        const dialog = page.panel()!.querySelector('[role="dialog"]')!;
        expect(dialog.getAttribute('aria-label')).toBeTruthy();
        expect(page.trigger().getAttribute('aria-controls')).toBe(dialog.id);
        expect(page.trigger().getAttribute('aria-expanded')).toBe('true');
    });

    it('offers "save current view" only to users who may create one', () => {
        canCreateIssue.set(false);
        const page = render([view()]);
        page.open();

        const labels = Array.from(page.panel()!.querySelectorAll('.sv-footer .sv-name')).map(node =>
            node.textContent!.trim()
        );
        expect(labels.some(label => label.includes('SAVE_CURRENT'))).toBe(false);
    });

    it('closing the panel resets the search and the confirm pin', () => {
        const page = render([view()]);
        page.open();
        page.component.query.set('bug');
        page.component.confirmingId.set(1);

        page.component.onPanelHide();

        expect(page.component.query()).toBe('');
        expect(page.component.confirmingId()).toBeNull();
        expect(page.component.isOpen()).toBe(false);
    });

    // Regression: focus was restored unconditionally, so a click on another toolbar control
    // landed and focus then jumped straight back to the trigger.
    it('does not reclaim focus when the panel never held it', () => {
        const page = render([view()]);
        const outside = document.createElement('input');
        document.body.appendChild(outside);
        page.open();
        outside.focus();

        page.component.onPanelHide();

        expect(document.activeElement).toBe(outside);
        outside.remove();
    });
});

import { Component } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { BehaviorSubject, Subject, of } from 'rxjs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { SavedViewApi } from 'src/app/project/api/saved-view.api.service';
import { SavedView } from 'src/app/project/model/saved-view.model';
import { ProjectStore } from 'src/app/project/project.store';
import { AclStore } from 'src/app/project/store/acl.store';
import { SavedViewStore } from 'src/app/project/store/saved-view.store';
import { UiToastService } from 'src/app/ui/service/ui-toast.service';
import { IssueToolbarService } from '../../issue-toolbar.service';
import { IssueViewMode } from '../../constants/issue-view-modes.enum';
import { SavedViewApplyService } from '../../service/saved-view-apply.service';
import { IssuePage } from './issue.page';

const ID_PROJECT = 5;

function view(over: Partial<SavedView> = {}): SavedView {
    return {
        idSavedView: 7,
        idProject: ID_PROJECT,
        name: 'My bugs',
        viewType: IssueViewMode.TABLE,
        isShared: false,
        createBy: 1,
        updateAt: '2026-08-01T00:00:00Z',
        config: { v: 1 },
        ...over
    } as SavedView;
}

function paramMapOf(params: Record<string, string>): { get: (key: string) => string | null } {
    return { get: key => params[key] ?? null };
}

@Component({ selector: 'app-saved-view-menu', template: '', standalone: true })
class SavedViewMenuStub {}

describe('IssuePage ?view= deep link (browser)', () => {
    let apply: ReturnType<typeof vi.fn>;
    let markUrl: ReturnType<typeof vi.fn>;
    let toastDetails: string[];
    let queryParamMap: Subject<{ get: (key: string) => string | null }>;
    let loadByProject$: ReturnType<typeof vi.fn>;
    let store: SavedViewStore;

    beforeEach(async () => {
        apply = vi.fn();
        markUrl = vi.fn();
        toastDetails = [];
        queryParamMap = new BehaviorSubject(paramMapOf({}));
        loadByProject$ = vi.fn(() => of([view()]));

        await TestBed.configureTestingModule({
            imports: [TranslateModule.forRoot(), SavedViewMenuStub],
            declarations: [IssuePage],
            providers: [
                { provide: SavedViewApi, useValue: { loadByProject$ } },
                {
                    provide: SavedViewApplyService,
                    useValue: { apply, markUrl, currentMode: () => IssueViewMode.TABLE }
                },
                { provide: ProjectStore, useValue: { project$: of({ idProject: ID_PROJECT }) } },
                { provide: IssueToolbarService, useValue: { toolbarTemplate: () => null } },
                { provide: AclStore, useValue: { canCreateIssue: () => true } },
                {
                    provide: UiToastService,
                    useValue: {
                        show: ({ detail }: { detail: string }) => toastDetails.push(detail)
                    }
                },
                { provide: ActivatedRoute, useValue: { queryParamMap } }
            ]
        })
            .overrideComponent(IssuePage, { set: { template: '', styles: [] } })
            .compileComponents();

        store = TestBed.inject(SavedViewStore);
    });

    function render(): ComponentFixture<IssuePage> {
        const fixture = TestBed.createComponent(IssuePage);
        fixture.detectChanges();
        return fixture;
    }

    it('applies the view named by the param', () => {
        render();

        queryParamMap.next(paramMapOf({ view: '7' }));

        expect(apply).toHaveBeenCalledWith(expect.objectContaining({ idSavedView: 7 }), ID_PROJECT);
    });

    it('does nothing without the param', () => {
        render();

        queryParamMap.next(paramMapOf({}));

        expect(apply).not.toHaveBeenCalled();
    });

    // apply() sets appliedId before navigating, so the param it writes must not come back
    // round as a second apply.
    it('does not re-apply the view it is already following', () => {
        render();
        store.setApplied(7);

        queryParamMap.next(paramMapOf({ view: '7' }));

        expect(apply).not.toHaveBeenCalled();
    });

    it('stops following the view when a plain navigation drops the param', () => {
        render();
        store.setApplied(7);
        expect(store.idAppliedView()).toBe(7);

        queryParamMap.next(paramMapOf({}));

        expect(store.idAppliedView()).toBeNull();
        expect(apply).not.toHaveBeenCalled();
    });

    it('reports an unknown id and drops the param', () => {
        render();

        queryParamMap.next(paramMapOf({ view: '999' }));

        expect(apply).not.toHaveBeenCalled();
        expect(toastDetails).toEqual(['ISSUE.VIEWS.NOT_FOUND']);
        expect(markUrl).toHaveBeenCalledWith(null);
    });

    // Regression: gating on views.length also matched "loaded, and there are none", so a
    // stale link in an empty project was ignored forever instead of being cleaned up.
    it('reports an unknown id even when the project has no views at all', () => {
        loadByProject$.mockReturnValue(of([]));
        render();

        queryParamMap.next(paramMapOf({ view: '999' }));

        expect(toastDetails).toEqual(['ISSUE.VIEWS.NOT_FOUND']);
        expect(markUrl).toHaveBeenCalledWith(null);
    });

    it('waits for the listing before judging the id', () => {
        const pending = new Subject<SavedView[]>();
        loadByProject$.mockReturnValue(pending);
        const fixture = render();

        queryParamMap.next(paramMapOf({ view: '7' }));
        expect(apply).not.toHaveBeenCalled();
        expect(toastDetails).toEqual([]);

        pending.next([view()]);
        fixture.detectChanges(); // loadedProject is a signal — flush it into the stream

        expect(apply).toHaveBeenCalledTimes(1);
    });
});

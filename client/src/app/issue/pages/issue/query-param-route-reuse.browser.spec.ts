import { Component, inject } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideLocationMocks } from '@angular/common/testing';
import {
    ActivatedRoute,
    Router,
    RouterOutlet,
    Routes,
    provideRouter,
    withRouterConfig
} from '@angular/router';

/**
 * Proves the router premise the saved-views apply rests on: changing only a query param
 * reuses the routed child (a recreated one would reinstall its hardcoded default filter and
 * clobber the applied view) and the parent still sees the new param. The route shape mirrors
 * the real one: project shell with resolvers → lazy issue → view → view-type children.
 */

const mounts: { tag: string; instance: unknown }[] = [];
const seenViewParam: (string | null)[] = [];

@Component({ selector: 'table-stub', standalone: true, template: '' })
class TableStub {
    public constructor() {
        mounts.push({ tag: 'table', instance: this });
    }
}

@Component({ selector: 'kanban-stub', standalone: true, template: '' })
class KanbanStub {
    public constructor() {
        mounts.push({ tag: 'kanban', instance: this });
    }
}

@Component({
    selector: 'issue-page-stub',
    standalone: true,
    imports: [RouterOutlet],
    template: '<router-outlet />'
})
class IssuePageStub {
    public constructor() {
        inject(ActivatedRoute).queryParamMap.subscribe(params =>
            seenViewParam.push(params.get('view'))
        );
    }
}

@Component({
    selector: 'shell-stub',
    standalone: true,
    imports: [RouterOutlet],
    template: '<router-outlet />'
})
class ShellStub {}

@Component({
    selector: 'root-stub',
    standalone: true,
    imports: [RouterOutlet],
    template: '<router-outlet />'
})
class RootStub {}

const routes: Routes = [
    {
        path: 'project/:idProject',
        component: ShellStub,
        resolve: { project: () => 'project' },
        children: [
            {
                path: 'issue',
                children: [
                    {
                        path: 'view',
                        component: IssuePageStub,
                        children: [
                            { path: 'table', component: TableStub },
                            { path: 'kanban', component: KanbanStub }
                        ]
                    }
                ]
            }
        ]
    }
];

describe('query-param-only navigation (saved-views premise)', () => {
    beforeEach(() => {
        mounts.length = 0;
        seenViewParam.length = 0;
        TestBed.configureTestingModule({
            providers: [
                provideRouter(routes, withRouterConfig({ paramsInheritanceStrategy: 'always' })),
                provideLocationMocks()
            ]
        });
    });

    async function mountTable() {
        const fixture = TestBed.createComponent(RootStub);
        const router = TestBed.inject(Router);
        await router.navigateByUrl('/project/1/issue/view/table');
        fixture.detectChanges();
        return { fixture, router };
    }

    it('reuses the routed child instance and still notifies the parent', async () => {
        const { fixture, router } = await mountTable();
        expect(mounts).toHaveLength(1);
        const firstInstance = mounts[0].instance;

        await router.navigate(['/project/1/issue/view/table'], { queryParams: { view: 7 } });
        fixture.detectChanges();

        expect(mounts).toHaveLength(1);
        expect(mounts[0].instance).toBe(firstInstance);
        expect(seenViewParam).toEqual([null, '7']);
    });

    // Negative control: without it the assertion above could pass vacuously, because the
    // harness never detects a remount at all.
    it('recreates the child when the view type changes', async () => {
        const { fixture, router } = await mountTable();

        await router.navigate(['/project/1/issue/view/kanban'], { queryParams: { view: 7 } });
        fixture.detectChanges();

        expect(mounts.map(mount => mount.tag)).toEqual(['table', 'kanban']);
    });
});

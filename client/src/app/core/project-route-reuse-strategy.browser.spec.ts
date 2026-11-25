import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import {
    RouteReuseStrategy,
    Router,
    RouterOutlet,
    provideRouter,
    withRouterConfig
} from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { describe, expect, it } from 'vitest';
import { ProjectRouteReuseStrategy } from './project-route-reuse-strategy';

const created: string[] = [];

@Component({ selector: 'app-layout-stub', template: '<router-outlet />', imports: [RouterOutlet] })
class LayoutStub {
    public constructor() {
        created.push('layout');
    }
}

@Component({ selector: 'app-page-stub', template: '' })
class PageStub {
    public constructor() {
        created.push('page');
    }
}

async function setup(): Promise<RouterTestingHarness> {
    created.length = 0;
    TestBed.configureTestingModule({
        providers: [
            provideRouter(
                [
                    {
                        path: 'project/:idProject',
                        component: LayoutStub,
                        children: [{ path: 'issue/:idIssuePublic', component: PageStub }]
                    }
                ],
                withRouterConfig({ paramsInheritanceStrategy: 'always' })
            ),
            { provide: RouteReuseStrategy, useClass: ProjectRouteReuseStrategy }
        ]
    });
    const harness = await RouterTestingHarness.create();
    await harness.navigateByUrl('/project/1/issue/A-1');
    created.length = 0;
    return harness;
}

describe('ProjectRouteReuseStrategy', () => {
    it('builds the project pages again when another project opens', async () => {
        const harness = await setup();

        await harness.navigateByUrl('/project/2/issue/A-1');

        expect(created).toEqual(['layout', 'page']);
    });

    it('keeps the pages when only something inside the project changes', async () => {
        const harness = await setup();

        await harness.navigateByUrl('/project/1/issue/A-2?tab=log');

        expect(created).toEqual([]);
        expect(TestBed.inject(Router).url).toBe('/project/1/issue/A-2?tab=log');
    });
});

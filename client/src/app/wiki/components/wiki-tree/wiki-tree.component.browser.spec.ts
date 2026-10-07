import { DragDropModule } from '@angular/cdk/drag-drop';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { RouterLink, provideRouter } from '@angular/router';
import { TranslateLoader, TranslateModule } from '@ngx-translate/core';
import { Observable, of } from 'rxjs';
import { describe, expect, it } from 'vitest';
import { TablerIconStub, UiTooltipStub } from 'src/testing/stubs';
import { WikiAgentAccess } from '../../constants/wiki-agent-access.enum';
import { WikiSpaceKind } from '../../constants/wiki-space-kind.enum';
import { WikiTreeConverter } from '../../converter/wiki-tree.converter';
import { WikiTreeNode } from '../../model/wiki-tree.model';
import { WikiTreeComponent } from './wiki-tree.component';

class EmptyLoader implements TranslateLoader {
    public getTranslation(): Observable<Record<string, string>> {
        return of({});
    }
}

const node = (
    idPage: number,
    idParent: number | null,
    title: string,
    agentAccess = WikiAgentAccess.OnDemand
): WikiTreeNode => ({
    idPage,
    idSpace: 2,
    idParent,
    slug: `page-${idPage}`,
    title,
    agentAccess,
    rank: `m${idPage}`
});

const nodes = [
    node(1, null, 'Operations'),
    node(2, 1, 'Deployment'),
    node(3, 2, 'Rollback when a database migration fails'),
    node(4, null, 'Conventions', WikiAgentAccess.Always)
];

async function setup(expanded: number[]): Promise<{
    fixture: ComponentFixture<WikiTreeComponent>;
    toggled: number[];
}> {
    await TestBed.configureTestingModule({
        declarations: [WikiTreeComponent],
        imports: [
            TranslateModule.forRoot({
                loader: { provide: TranslateLoader, useClass: EmptyLoader }
            }),
            DragDropModule,
            RouterLink,
            TablerIconStub,
            UiTooltipStub
        ],
        providers: [provideRouter([])]
    }).compileComponents();
    const fixture = TestBed.createComponent(WikiTreeComponent);
    fixture.componentRef.setInput('entries', WikiTreeConverter.toEntries(nodes, 2));
    fixture.componentRef.setInput('idProject', 7);
    fixture.componentRef.setInput('spaceKind', WikiSpaceKind.Project);
    fixture.componentRef.setInput('expanded', new Set(expanded));
    fixture.componentRef.setInput('activeIdPage', 3);
    const toggled: number[] = [];
    fixture.componentInstance.toggled.subscribe(id => toggled.push(id));
    fixture.detectChanges();
    return { fixture, toggled };
}

function titles(fixture: ComponentFixture<WikiTreeComponent>): string[] {
    return Array.from(
        (fixture.nativeElement as HTMLElement).querySelectorAll('[data-testid="wiki-tree-node"]')
    ).map(element => {
        const title = element.querySelector('.wiki-tree__title')?.textContent?.trim() ?? '';
        const count = element.querySelector('.wiki-tree__count')?.textContent?.trim();
        return count ? `${title} (${count})` : title;
    });
}

describe('WikiTreeComponent', () => {
    it('shows only root pages while folders are collapsed, with the subtree size', async () => {
        const { fixture } = await setup([]);
        expect(titles(fixture)).toEqual(['Operations (2)', 'Conventions']);
    });

    it('reveals nested pages of expanded folders and links each page to its route', async () => {
        const { fixture } = await setup([1, 2]);
        expect(titles(fixture)).toEqual([
            'Operations (2)',
            'Deployment (1)',
            'Rollback when a database migration fails',
            'Conventions'
        ]);
        const links = (fixture.nativeElement as HTMLElement).querySelectorAll('a');
        expect(links[2].getAttribute('href')).toBe('/project/7/wiki/project/page-3');
        expect(links[2].classList).toContain('wiki-tree__node--active');
    });

    it('toggles a folder without navigating to it', async () => {
        const { fixture, toggled } = await setup([]);
        const chevron = (fixture.nativeElement as HTMLElement).querySelector<HTMLButtonElement>(
            '.wiki-tree__chevron'
        );
        chevron?.click();
        expect(toggled).toEqual([1]);
    });
});

import { By } from '@angular/platform-browser';
import { Component, input } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { MarkdownModule } from 'ngx-markdown';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MermaidDiagramComponent } from 'src/app/shared/mermaid/mermaid-diagram.component';
import { MermaidLoaderService } from 'src/app/shared/mermaid/mermaid-loader.service';
import { WikiMermaidDirective } from './wiki-mermaid.directive';

@Component({
    standalone: false,
    template: `
        <div markdown appWikiMermaid [data]="body()"></div>
    `
})
class HostComponent {
    public readonly body = input('');
}

describe('WikiMermaidDirective (browser)', () => {
    let loader: { load: ReturnType<typeof vi.fn> };

    beforeEach(async () => {
        loader = {
            load: vi
                .fn()
                .mockResolvedValue({ render: vi.fn().mockResolvedValue({ svg: '<svg></svg>' }) })
        };
        await TestBed.configureTestingModule({
            declarations: [HostComponent, WikiMermaidDirective, MermaidDiagramComponent],
            imports: [MarkdownModule.forRoot(), TranslateModule.forRoot()],
            providers: [{ provide: MermaidLoaderService, useValue: loader }]
        }).compileComponents();
    });

    function create(body: string): ComponentFixture<HostComponent> {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.componentRef.setInput('body', body);
        return fixture;
    }

    async function render(body: string) {
        const fixture = create(body);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();
        return fixture;
    }

    function directive(fixture: ComponentFixture<HostComponent>): WikiMermaidDirective {
        return fixture.debugElement
            .query(By.directive(WikiMermaidDirective))
            .injector.get(WikiMermaidDirective);
    }

    it('does not load mermaid when the page has no diagram', async () => {
        const fixture = await render('# Title\n\nJust text, no fence.');
        expect(fixture.nativeElement.querySelector('app-mermaid-diagram')).toBeNull();
        expect(loader.load).not.toHaveBeenCalled();
    });

    it('inserts an app-mermaid-diagram in place of the mermaid fence', async () => {
        const fixture = await render(
            'Intro\n\n```mermaid\nflowchart TD\n    A --> B\n```\n\nTail.'
        );
        const diagram = fixture.nativeElement.querySelector('app-mermaid-diagram');
        expect(diagram).not.toBeNull();
        // The rendered code fence is gone; the loading-state <pre> inside the
        // component is its own markup, so assert on the fence specifically.
        expect(fixture.nativeElement.querySelector('pre > code.language-mermaid')).toBeNull();
        expect(fixture.nativeElement.textContent).toContain('Intro');
    });

    it('emits settled after every diagram of a markdown render has settled', async () => {
        const settled = vi.fn();
        const body =
            '```mermaid\nflowchart TD\n    A --> B\n```\n' +
            'text between\n' +
            '```mermaid\nflowchart LR\n    C --> D\n```';
        const fixture = create(body);
        directive(fixture).settled.subscribe(settled);
        fixture.detectChanges();

        await vi.waitFor(() => expect(settled).toHaveBeenCalledTimes(1));
        await fixture.whenStable();

        const diagrams = fixture.nativeElement.querySelectorAll('app-mermaid-diagram');
        expect(diagrams.length).toBe(2);
        expect(loader.load).toHaveBeenCalled();
    });

    it('emits settled again when the markdown re-renders with another diagram', async () => {
        const settled = vi.fn();
        const fixture = create('```mermaid\nflowchart TD\n    A --> B\n```');
        directive(fixture).settled.subscribe(settled);
        fixture.detectChanges();
        await vi.waitFor(() => expect(settled).toHaveBeenCalledTimes(1));
        await fixture.whenStable();

        fixture.componentRef.setInput('body', '```mermaid\nflowchart LR\n    B --> C\n```');
        await vi.waitFor(() => expect(settled).toHaveBeenCalledTimes(2));
        await fixture.whenStable();
    });

    it('does not emit settled for a markdown render without diagrams', async () => {
        const settled = vi.fn();
        const fixture = create('# Just a heading');
        directive(fixture).settled.subscribe(settled);
        fixture.detectChanges();
        await fixture.whenStable();
        expect(settled).not.toHaveBeenCalled();
    });
});

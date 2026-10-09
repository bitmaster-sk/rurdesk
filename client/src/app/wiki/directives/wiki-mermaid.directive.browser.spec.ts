import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { MarkdownModule } from 'ngx-markdown';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MermaidDiagramComponent } from 'src/app/shared/mermaid/mermaid-diagram.component';
import { MermaidLoaderService } from 'src/app/shared/mermaid/mermaid-loader.service';
import { WikiMermaidDirective } from './wiki-mermaid.directive';

@Component({
    standalone: false,
    template: `
        <div markdown appWikiMermaid [data]="body"></div>
    `
})
class HostComponent {
    public body = '';
}

describe('WikiMermaidDirective (browser)', () => {
    let loader: { load: ReturnType<typeof vi.fn> };

    beforeEach(async () => {
        // Backing fake mermaid so a diagram component whose debounce fires
        // inside the test still resolves instead of crashing on a stub.
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

    async function render(body: string) {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.componentInstance.body = body;
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();
        return fixture;
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
});

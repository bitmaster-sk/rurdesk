import { Component, input } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { MarkdownComponent, MarkdownModule } from 'ngx-markdown';
import { TranslateModule } from '@ngx-translate/core';
import { of } from 'rxjs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ClipboardService } from 'src/app/core/clipboard.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { MermaidDiagramComponent } from 'src/app/shared/mermaid/mermaid-diagram.component';
import { MermaidLoaderService } from 'src/app/shared/mermaid/mermaid-loader.service';
import { PlantUmlApi } from 'src/app/shared/plantuml/plantuml.api.service';
import { PlantUmlDiagramComponent } from 'src/app/shared/plantuml/plantuml-diagram.component';
import { WikiCodeCopyDirective } from './wiki-code-copy.directive';
import { WikiMermaidDirective } from './wiki-mermaid.directive';
import { WikiPlantUmlDirective } from './wiki-plantuml.directive';

@Component({
    standalone: false,
    template: `
        <div markdown appWikiCodeCopy [data]="body()"></div>
    `
})
class HostComponent {
    public readonly body = input('');
}

/** Mirrors the directive order of the wiki page and editor preview articles. */
@Component({
    standalone: false,
    template: `
        <div markdown appWikiMermaid appWikiPlantUml appWikiCodeCopy [data]="body()"></div>
    `
})
class DiagramHostComponent {
    public readonly body = input('');
}

describe('WikiCodeCopyDirective (browser)', () => {
    let copy: ReturnType<typeof vi.fn>;
    let showSuccess: ReturnType<typeof vi.fn>;
    let showError: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        copy = vi.fn().mockResolvedValue(true);
        showSuccess = vi.fn();
        showError = vi.fn();
        await TestBed.configureTestingModule({
            declarations: [
                HostComponent,
                DiagramHostComponent,
                WikiCodeCopyDirective,
                WikiMermaidDirective,
                WikiPlantUmlDirective,
                MermaidDiagramComponent,
                PlantUmlDiagramComponent
            ],
            imports: [MarkdownModule.forRoot(), TranslateModule.forRoot()],
            providers: [
                { provide: ClipboardService, useValue: { copy } },
                { provide: ToastNotificationService, useValue: { showSuccess, showError } },
                {
                    provide: I18nService,
                    useValue: {
                        instant: (key: string) =>
                            key === 'WIKI.PAGE.COPY_CODE' ? 'Copy code' : key
                    }
                },
                {
                    provide: MermaidLoaderService,
                    useValue: {
                        load: vi.fn().mockResolvedValue({
                            render: vi.fn().mockResolvedValue({ svg: '<svg></svg>' })
                        })
                    }
                },
                {
                    provide: PlantUmlApi,
                    useValue: { render$: vi.fn().mockReturnValue(of({ svg: '<svg></svg>' })) }
                },
                { provide: SettingsStore, useValue: { isPlantUmlEnabled: () => true } }
            ]
        }).compileComponents();
    });

    async function render(body: string): Promise<ComponentFixture<HostComponent>> {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.componentRef.setInput('body', body);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();
        return fixture;
    }

    function copyButtons(fixture: ComponentFixture<unknown>): HTMLButtonElement[] {
        return Array.from(fixture.nativeElement.querySelectorAll('[data-testid="wiki-code-copy"]'));
    }

    async function click(button: HTMLButtonElement): Promise<void> {
        button.click();
        await vi.waitFor(() => expect(copy).toHaveBeenCalled());
    }

    it('renders a real button with a translated label outside the code element', async () => {
        const fixture = await render('```\nalpha\n```');
        const button = copyButtons(fixture)[0];
        expect(button.tagName).toBe('BUTTON');
        expect(button.type).toBe('button');
        expect(button.textContent).toBe('Copy code');
        // Keyboard and touch reach it like any button: focusable, no overlay.
        button.focus();
        expect(document.activeElement).toBe(button);
        expect(button.closest('code')).toBeNull();
    });

    it('copies the exact multi-line text of a block without a language tag', async () => {
        const fixture = await render('```\nline one\n    indented line\nlast line\n```');
        await click(copyButtons(fixture)[0]);
        expect(copy).toHaveBeenCalledWith('line one\n    indented line\nlast line\n');
        expect(showSuccess).toHaveBeenCalledWith('WIKI.PAGE.CODE_COPIED');
    });

    it('copies HTML-escaped characters decoded back to the source', async () => {
        const fixture = await render('```html\n<div class="x">& "quoted" \'single\'</div>\n```');
        await click(copyButtons(fixture)[0]);
        expect(copy).toHaveBeenCalledWith('<div class="x">& "quoted" \'single\'</div>\n');
    });

    it('gives every block its own button and copies each block separately', async () => {
        const fixture = await render('```ts\nfirst\n```\n\ntext\n\n```ts\nsecond\n```');
        const buttons = copyButtons(fixture);
        expect(buttons.length).toBe(2);
        await click(buttons[0]);
        expect(copy).toHaveBeenCalledWith('first\n');
        await click(buttons[1]);
        expect(copy).toHaveBeenCalledWith('second\n');
    });

    it('re-decorates without duplicating buttons when the markdown re-renders', async () => {
        const fixture = await render('```\nalpha\n```');
        expect(copyButtons(fixture).length).toBe(1);

        fixture.componentRef.setInput('body', '```js\nbravo\n```\n\n```js\ncharlie\n```');
        fixture.detectChanges();
        await fixture.whenStable();

        expect(copyButtons(fixture).length).toBe(2);
        await click(copyButtons(fixture)[1]);
        expect(copy).toHaveBeenCalledWith('charlie\n');
        expect(copy).not.toHaveBeenCalledWith('alpha\n');
    });

    it('does not add a second button when ready fires again without a re-render', async () => {
        const fixture = await render('```\nalpha\n```');
        const markdown = fixture.debugElement
            .query(By.directive(MarkdownComponent))
            .injector.get(MarkdownComponent);
        markdown.ready.emit();
        markdown.ready.emit();

        expect(copyButtons(fixture).length).toBe(1);
        // The button of the first pass still carries its listener.
        await click(copyButtons(fixture)[0]);
        expect(copy).toHaveBeenCalledWith('alpha\n');
    });

    it('confirms success only when the clipboard reports true', async () => {
        copy.mockResolvedValueOnce(false);
        const fixture = await render('```\nalpha\n```');
        await click(copyButtons(fixture)[0]);
        await new Promise(resolve => setTimeout(resolve, 10));
        expect(showSuccess).not.toHaveBeenCalled();
        expect(showError).toHaveBeenCalledWith('WIKI.PAGE.CODE_COPY_FAILED');
    });

    it('leaves inline code without a button and untouched', async () => {
        const fixture = await render('Text with `inline code` inside a sentence.');
        expect(copyButtons(fixture).length).toBe(0);
        const code = fixture.nativeElement.querySelector('p code');
        expect(code).not.toBeNull();
        expect(code.parentElement.className).not.toContain('wiki-code');
    });

    it('ignores mermaid and plantuml fences by their language class', async () => {
        const fixture = await render(
            '```mermaid\nflowchart TD\n    A --> B\n```\n\n```plantuml\nAlice -> Bob: hi\n```\n\n```\nplain\n```'
        );
        expect(copyButtons(fixture).length).toBe(1);
    });

    it('leaves diagram fences to the diagram directives and keeps them rendering', async () => {
        const fixture = TestBed.createComponent(DiagramHostComponent);
        fixture.componentRef.setInput(
            'body',
            '```ts\nplain\n```\n\n```mermaid\nflowchart TD\n    A --> B\n```\n\n```plantuml\nAlice -> Bob: hi\n```'
        );
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelector('app-mermaid-diagram')).not.toBeNull();
        expect(fixture.nativeElement.querySelector('app-plantuml-diagram')).not.toBeNull();
        const buttons = copyButtons(fixture);
        expect(buttons.length).toBe(1);
        await click(buttons[0]);
        expect(copy).toHaveBeenCalledWith('plain\n');
    });
});

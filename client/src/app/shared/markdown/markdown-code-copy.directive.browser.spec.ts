import { Component, input } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import {
    IconCopy,
    IconCopyCheck,
    TablerIconComponent,
    provideTablerIcons
} from '@tabler/icons-angular';
import { MarkdownComponent, MarkdownModule } from 'ngx-markdown';
import { LangChangeEvent, TranslateModule } from '@ngx-translate/core';
import { of, Subject } from 'rxjs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ClipboardService } from 'src/app/core/clipboard.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { MermaidDiagramComponent } from 'src/app/shared/mermaid/mermaid-diagram.component';
import { MermaidLoaderService } from 'src/app/shared/mermaid/mermaid-loader.service';
import { PlantUmlApi } from 'src/app/shared/plantuml/plantuml.api.service';
import { PlantUmlDiagramComponent } from 'src/app/shared/plantuml/plantuml-diagram.component';
import { MarkdownCodeCopyDirective } from './markdown-code-copy.directive';
import { WikiMermaidDirective } from '../../wiki/directives/wiki-mermaid.directive';
import { WikiPlantUmlDirective } from '../../wiki/directives/wiki-plantuml.directive';

@Component({
    standalone: false,
    template: `
        <div markdown appMarkdownCodeCopy [data]="body()"></div>
    `
})
class HostComponent {
    public readonly body = input('');
}

/** Mirrors the directive order of wiki page and editor preview articles. */
@Component({
    standalone: false,
    template: `
        <div
            markdown
            appWikiMermaid
            appWikiPlantUml
            appMarkdownCodeCopy
            [skipDiagramSource]="true"
            [data]="body()"
        ></div>
    `
})
class DiagramHostComponent {
    public readonly body = input('');
}

describe('MarkdownCodeCopyDirective (browser)', () => {
    let copy: ReturnType<typeof vi.fn>;
    let showSuccess: ReturnType<typeof vi.fn>;
    let showError: ReturnType<typeof vi.fn>;
    let labels: Record<string, string>;
    let languageChanges: Subject<LangChangeEvent>;

    beforeEach(async () => {
        copy = vi.fn().mockResolvedValue(true);
        showSuccess = vi.fn();
        showError = vi.fn();
        labels = {
            'MARKDOWN.COPY_CODE': 'Copy code',
            'MARKDOWN.CODE_COPIED': 'Code copied',
            'MARKDOWN.CODE_COPY_FAILED': 'Could not copy the code'
        };
        languageChanges = new Subject<LangChangeEvent>();
        await TestBed.configureTestingModule({
            declarations: [
                HostComponent,
                DiagramHostComponent,
                MarkdownCodeCopyDirective,
                WikiMermaidDirective,
                WikiPlantUmlDirective,
                MermaidDiagramComponent,
                PlantUmlDiagramComponent
            ],
            imports: [MarkdownModule.forRoot(), TranslateModule.forRoot(), TablerIconComponent],
            providers: [
                provideTablerIcons({ IconCopy, IconCopyCheck }),
                { provide: ClipboardService, useValue: { copy } },
                { provide: ToastNotificationService, useValue: { showSuccess, showError } },
                {
                    provide: I18nService,
                    useValue: {
                        get$: (key: string) => of(labels[key] ?? key),
                        langChange$: languageChanges.asObservable()
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

    function copyIcons(fixture: ComponentFixture<unknown>): HTMLElement[] {
        return Array.from(
            fixture.nativeElement.querySelectorAll('[data-testid="markdown-code-copy"]')
        );
    }

    async function click(copyIcon: HTMLElement): Promise<void> {
        copyIcon.click();
        await vi.waitFor(() => expect(copy).toHaveBeenCalled());
    }

    it('renders a keyboard-accessible icon, not a button, inside the code block', async () => {
        const fixture = await render('```\nalpha\n```');
        const copyIcon = copyIcons(fixture)[0];
        expect(copyIcon.tagName).toBe('TABLER-ICON');
        expect(copyIcon.getAttribute('role')).toBe('button');
        expect(copyIcon.getAttribute('tabindex')).toBe('0');
        expect(copyIcon.getAttribute('aria-label')).toBe('Copy code');
        expect(copyIcon.querySelector('svg')).not.toBeNull();
        copyIcon.focus();
        expect(document.activeElement).toBe(copyIcon);
        expect(copyIcon.closest('.markdown-code-block')).not.toBeNull();
    });

    it('copies the block when the icon is activated from the keyboard', async () => {
        const fixture = await render('```\nalpha\n```');
        const copyIcon = copyIcons(fixture)[0];
        copyIcon.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        await vi.waitFor(() => expect(copy).toHaveBeenCalledWith('alpha\n'));
    });

    it('copies the exact multi-line text of a block without a language tag', async () => {
        const fixture = await render('```\nline one\n    indented line\nlast line\n```');
        const copyIcon = copyIcons(fixture)[0];
        const copySvg = copyIcon.querySelector('svg')?.innerHTML;
        const onParentClick = vi.fn();
        fixture.nativeElement.addEventListener('click', onParentClick);
        await click(copyIcon);
        expect(onParentClick).not.toHaveBeenCalled();
        expect(copy).toHaveBeenCalledWith('line one\n    indented line\nlast line\n');
        expect(showSuccess).toHaveBeenCalledWith('MARKDOWN.CODE_COPIED');
        await vi.waitFor(() => {
            expect(copyIcon.getAttribute('aria-label')).toBe('Code copied');
            expect(copyIcon.querySelector('svg')?.innerHTML).not.toBe(copySvg);
        });
    });

    it('updates the copied label using the active translation', async () => {
        const fixture = await render('```\nalpha\n```');
        const copyIcon = copyIcons(fixture)[0];
        await click(copyIcon);
        labels['MARKDOWN.CODE_COPIED'] = 'Localized code copied';
        languageChanges.next({ lang: 'localized', translations: {} });

        await vi.waitFor(() => {
            expect(copyIcon.getAttribute('aria-label')).toBe('Localized code copied');
            expect(copyIcon.title).toBe('Localized code copied');
        });
    });

    it('copies HTML-escaped characters decoded back to the source', async () => {
        const fixture = await render('```html\n<div class="x">& "quoted" \'single\'</div>\n```');
        await click(copyIcons(fixture)[0]);
        expect(copy).toHaveBeenCalledWith('<div class="x">& "quoted" \'single\'</div>\n');
    });

    it('gives every block its own button and copies each block separately', async () => {
        const fixture = await render('```ts\nfirst\n```\n\ntext\n\n```ts\nsecond\n```');
        const copyIconsForBlocks = copyIcons(fixture);
        expect(copyIconsForBlocks.length).toBe(2);
        await click(copyIconsForBlocks[0]);
        expect(copy).toHaveBeenCalledWith('first\n');
        await click(copyIconsForBlocks[1]);
        expect(copy).toHaveBeenCalledWith('second\n');
    });

    it('re-decorates without duplicating buttons when the markdown re-renders', async () => {
        const fixture = await render('```\nalpha\n```');
        expect(copyIcons(fixture).length).toBe(1);

        fixture.componentRef.setInput('body', '```js\nbravo\n```\n\n```js\ncharlie\n```');
        fixture.detectChanges();
        await fixture.whenStable();

        expect(copyIcons(fixture).length).toBe(2);
        await click(copyIcons(fixture)[1]);
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

        expect(copyIcons(fixture).length).toBe(1);
        // The host-level listener still handles the original icon.
        await click(copyIcons(fixture)[0]);
        expect(copy).toHaveBeenCalledWith('alpha\n');
    });

    it('confirms success only when the clipboard reports true', async () => {
        copy.mockResolvedValueOnce(false);
        const fixture = await render('```\nalpha\n```');
        await click(copyIcons(fixture)[0]);
        await new Promise(resolve => setTimeout(resolve, 10));
        expect(showSuccess).not.toHaveBeenCalled();
        expect(showError).toHaveBeenCalledWith('MARKDOWN.CODE_COPY_FAILED');
        expect(copyIcons(fixture)[0].getAttribute('aria-label')).toBe('Copy code');
    });

    it('leaves inline code without a button and untouched', async () => {
        const fixture = await render('Text with `inline code` inside a sentence.');
        expect(copyIcons(fixture).length).toBe(0);
        const code = fixture.nativeElement.querySelector('p code');
        expect(code).not.toBeNull();
        expect(code.parentElement.className).not.toContain('markdown-code-block');
    });

    it('adds copy icons to diagram-tagged blocks when the host does not render diagrams', async () => {
        const fixture = await render(
            '```mermaid\nflowchart TD\n    A --> B\n```\n\n```plantuml\nAlice -> Bob: hi\n```\n\n```\nplain\n```'
        );
        expect(copyIcons(fixture).length).toBe(3);
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
        const copyIconsForBlocks = copyIcons(fixture);
        expect(copyIconsForBlocks.length).toBe(1);
        await click(copyIconsForBlocks[0]);
        expect(copy).toHaveBeenCalledWith('plain\n');
    });
});

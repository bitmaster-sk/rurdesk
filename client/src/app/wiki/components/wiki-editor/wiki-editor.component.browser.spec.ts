import { Component, input } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { EditorView } from '@codemirror/view';
import { deleteCharBackward } from '@codemirror/commands';
import { TranslateLoader, TranslateModule } from '@ngx-translate/core';
import { Observable, of } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { TablerIconStub, UiButtonStub, UiLoaderStub, UiTooltipStub } from 'src/testing/stubs';
import { IssueLinkPickerComponent } from 'src/app/link/components/issue-link-picker/issue-link-picker.component';
import { IssueLinkApi } from 'src/app/link/api/issue-link.api.service';
import { IssueLinkOption } from 'src/app/link/model/issue-link.model';
import { LinkScope } from 'src/app/link/service/link-scope.service';
import { UiMenuItem } from 'src/app/ui/components/menu/menu-item.model';
import { WikiTablePickerComponent } from '../wiki-table-picker/wiki-table-picker.component';
import { WikiEditorComponent } from './wiki-editor.component';

@Component({ selector: 'ui-popover', template: '<ng-content></ng-content>', standalone: true })
class UiPopoverStub {
    public toggle(): void {}
    public hide(): void {}
}

@Component({ selector: 'ui-menu', template: '', standalone: true })
class UiMenuStub {
    public readonly model = input<UiMenuItem[]>([]);
    public toggle(): void {}
}

class EmptyLoader implements TranslateLoader {
    public getTranslation(): Observable<Record<string, string>> {
        return of({});
    }
}

async function render(
    text: string
): Promise<{ element: HTMLElement; view: EditorView; values: string[]; menu: UiMenuItem[] }> {
    await TestBed.configureTestingModule({
        declarations: [WikiEditorComponent, WikiTablePickerComponent, IssueLinkPickerComponent],
        imports: [
            TranslateModule.forRoot({
                loader: { provide: TranslateLoader, useClass: EmptyLoader }
            }),
            TablerIconStub,
            UiButtonStub,
            UiTooltipStub,
            UiLoaderStub,
            UiPopoverStub,
            UiMenuStub
        ],
        providers: [{ provide: IssueLinkApi, useValue: { load$: () => of([]) } }, LinkScope]
    }).compileComponents();
    const fixture = TestBed.createComponent(WikiEditorComponent);
    fixture.componentRef.setInput('initialValue', text);
    const values: string[] = [];
    fixture.componentInstance.valueChange.subscribe(value => values.push(value));
    fixture.detectChanges();
    const element = fixture.nativeElement as HTMLElement;
    const view = EditorView.findFromDOM(element.querySelector<HTMLElement>('.cm-editor')!)!;
    const menu = fixture.debugElement.query(debug => debug.componentInstance instanceof UiMenuStub)
        .componentInstance as UiMenuStub;
    return { element, view, values, menu: menu.model() };
}

function click(element: HTMLElement, testId: string): void {
    element.querySelector<HTMLElement>(`[data-testid="${testId}"]`)!.click();
}

describe('WikiEditorComponent toolbar', () => {
    it('makes the selected word bold and reports the new text', async () => {
        const { element, view, values } = await render('make it loud');
        view.dispatch({ selection: { anchor: 8, head: 12 } });

        click(element, 'wiki-format-bold');

        expect(values.at(-1)).toBe('make it **loud**');
    });

    it('turns the line with the cursor into a heading', async () => {
        const { element, view, values } = await render('Intro\nSetup');
        view.dispatch({ selection: { anchor: 8 } });

        click(element, 'wiki-format-heading2');

        expect(values.at(-1)).toBe('Intro\n## Setup');
    });

    it('applies bold from the keyboard shortcut too', async () => {
        const { view, values } = await render('make it loud');
        view.dispatch({ selection: { anchor: 8, head: 12 } });
        view.focus();

        const isMac = /Mac/.test(navigator.platform);
        view.contentDOM.dispatchEvent(
            new KeyboardEvent('keydown', {
                key: 'b',
                ctrlKey: !isMac,
                metaKey: isMac,
                bubbles: true
            })
        );

        expect(values.at(-1)).toBe('make it **loud**');
    });

    it('inserts a table of the size picked in the grid', async () => {
        const { element, view, values } = await render('');
        view.dispatch({ selection: { anchor: 0 } });

        click(element, 'wiki-table-cell-2-3');

        expect(values.at(-1)).toBe(
            '| Column 1 | Column 2 | Column 3 |\n| --- | --- | --- |\n|   |   |   |'
        );
    });

    it('wraps the current line in a warning block from the callout menu', async () => {
        const { view, values, menu } = await render('Run migrations first');
        view.dispatch({ selection: { anchor: 3 } });

        menu.find(item => item.labelKey === 'WIKI.CALLOUT.WARNING')?.command?.();

        expect(values.at(-1)).toBe('> [!WARNING]\n> Run migrations first');
    });
});

const issueOptions: IssueLinkOption[] = [
    { idProject: 7, projectName: 'Rurdesk', idIssuePublic: 123, title: 'Fix login' },
    { idProject: 9, projectName: 'Website', idIssuePublic: 12, title: 'Blog redesign' }
];

describe('WikiEditorComponent issue links', () => {
    async function renderIssueLinks(text: string): Promise<{
        element: HTMLElement;
        view: EditorView;
        load: ReturnType<typeof vi.fn>;
        fixture: ComponentFixture<WikiEditorComponent>;
    }> {
        const load = vi.fn().mockReturnValue(of(issueOptions));
        await TestBed.configureTestingModule({
            declarations: [WikiEditorComponent, WikiTablePickerComponent, IssueLinkPickerComponent],
            imports: [
                TranslateModule.forRoot({
                    loader: { provide: TranslateLoader, useClass: EmptyLoader }
                }),
                TablerIconStub,
                UiButtonStub,
                UiTooltipStub,
                UiLoaderStub,
                UiPopoverStub,
                UiMenuStub
            ],
            providers: [{ provide: IssueLinkApi, useValue: { load$: load } }, LinkScope]
        }).compileComponents();
        TestBed.inject(LinkScope).idProject.set(7);
        const fixture = TestBed.createComponent(WikiEditorComponent);
        fixture.componentRef.setInput('initialValue', text);
        fixture.detectChanges();
        const element = fixture.nativeElement as HTMLElement;
        const view = EditorView.findFromDOM(element.querySelector<HTMLElement>('.cm-editor')!)!;
        return { element, view, load, fixture };
    }

    async function typeAtEnd(
        fixture: ComponentFixture<WikiEditorComponent>,
        view: EditorView,
        text: string
    ): Promise<void> {
        const end = view.state.doc.length;
        view.focus();
        view.dispatch({
            changes: { from: end, to: end, insert: text },
            selection: { anchor: end + text.length }
        });
        await new Promise(resolve => setTimeout(resolve, 200));
        fixture.detectChanges();
    }

    function optionTexts(element: HTMLElement): string[] {
        return Array.from(element.querySelectorAll('[data-testid="issue-link-option"]')).map(
            option => option.textContent?.replace(/\s+/g, ' ').trim() ?? ''
        );
    }

    function press(view: EditorView, key: string): void {
        view.contentDOM.dispatchEvent(
            new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true })
        );
    }

    it('shows the same task picker as the message editor after #, current project first', async () => {
        const { element, view, load, fixture } = await renderIssueLinks('fixed in ');

        await typeAtEnd(fixture, view, '#lo');

        expect(load).toHaveBeenCalledWith('lo', 7);
        expect(
            element.querySelector('app-issue-link-picker [data-testid="issue-link-picker"]')
        ).not.toBeNull();
        expect(optionTexts(element)).toEqual([
            'Rurdesk #123 Fix login',
            'Website #12 Blog redesign'
        ]);
        expect(element.querySelector('.cm-tooltip-autocomplete')).toBeNull();
    });

    it('inserts the picked task as a full link that shows only its number', async () => {
        const { element, view, fixture } = await renderIssueLinks('fixed in ');
        await typeAtEnd(fixture, view, '#');

        press(view, 'ArrowDown');
        press(view, 'Enter');
        fixture.detectChanges();

        expect(view.state.doc.toString()).toBe('fixed in [#12](/project/9/issue/12) ');
        expect(element.querySelector('.cm-issue-link')?.textContent).toBe('#12');
        expect(optionTexts(element)).toEqual([]);
    });

    it('inserts the task clicked in the picker', async () => {
        const { element, view, fixture } = await renderIssueLinks('fixed in ');
        await typeAtEnd(fixture, view, '#');

        element
            .querySelector('[data-testid="issue-link-option"]')!
            .dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true }));
        fixture.detectChanges();

        expect(view.state.doc.toString()).toBe('fixed in [#123](/project/7/issue/123) ');
    });

    it('closes the picker on Escape and keeps the text', async () => {
        const { element, view, fixture } = await renderIssueLinks('fixed in ');
        await typeAtEnd(fixture, view, '#lo');

        press(view, 'Escape');
        fixture.detectChanges();

        expect(optionTexts(element)).toEqual([]);
        expect(view.state.doc.toString()).toBe('fixed in #lo');
    });

    it('deletes a task link as a whole', async () => {
        const text = 'see [#123](/project/7/issue/123)';
        const { view } = await renderIssueLinks(text);

        view.dispatch({ selection: { anchor: text.length } });
        deleteCharBackward(view);

        expect(view.state.doc.toString()).toBe('see ');
    });
});

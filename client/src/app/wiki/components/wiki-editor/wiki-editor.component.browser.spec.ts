import { Component, input } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { EditorView } from '@codemirror/view';
import { TranslateLoader, TranslateModule } from '@ngx-translate/core';
import { Observable, of } from 'rxjs';
import { describe, expect, it } from 'vitest';
import { TablerIconStub, UiButtonStub, UiTooltipStub } from 'src/testing/stubs';
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
        declarations: [WikiEditorComponent, WikiTablePickerComponent],
        imports: [
            TranslateModule.forRoot({
                loader: { provide: TranslateLoader, useClass: EmptyLoader }
            }),
            TablerIconStub,
            UiButtonStub,
            UiTooltipStub,
            UiPopoverStub,
            UiMenuStub
        ]
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

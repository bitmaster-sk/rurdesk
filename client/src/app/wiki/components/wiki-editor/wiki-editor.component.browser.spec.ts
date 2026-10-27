import { Component, input } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { EditorView } from '@codemirror/view';
import { startCompletion, closeCompletion } from '@codemirror/autocomplete';
import { TranslateLoader, TranslateModule } from '@ngx-translate/core';
import { Observable, Subject, of } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { TablerIconStub, UiButtonStub, UiTooltipStub } from 'src/testing/stubs';
import { IssueApi } from 'src/app/issue/api/issue.api.service';
import { Issue } from 'src/app/issue/model/issue.model';
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
        ],
        providers: [
            { provide: IssueApi, useValue: { load$: () => new Subject<Issue[]>() } },
            LinkScope
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

const issueOption = (idIssuePublic: number, title: string): Issue => ({
    idIssue: idIssuePublic,
    idIssuePublic,
    idProject: 7,
    idState: null,
    idSeverity: null,
    idIssueType: null,
    title,
    description: '',
    tracked: 0
});

describe('WikiEditorComponent issue picker', () => {
    async function renderIssuePicker(issues$: Subject<Issue[]>): Promise<{
        element: HTMLElement;
        view: EditorView;
        load: ReturnType<typeof vi.fn>;
    }> {
        const load = vi.fn().mockReturnValue(issues$);
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
            ],
            providers: [{ provide: IssueApi, useValue: { load$: load } }, LinkScope]
        }).compileComponents();
        TestBed.inject(LinkScope).idProject.set(7);
        const fixture = TestBed.createComponent(WikiEditorComponent);
        fixture.componentRef.setInput('initialValue', 'fixed in ');
        fixture.detectChanges();
        const element = fixture.nativeElement as HTMLElement;
        const view = EditorView.findFromDOM(element.querySelector<HTMLElement>('.cm-editor')!)!;
        return { element, view, load };
    }

    function tooltipOptions(element: HTMLElement): string[] {
        return Array.from(element.querySelectorAll('.cm-completionLabel')).map(
            option => option.textContent?.trim() ?? ''
        );
    }

    async function openPicker(view: EditorView): Promise<void> {
        startCompletion(view);
        await new Promise(resolve => setTimeout(resolve, 120));
    }

    it('takes the # options from the issue link store and refreshes them on every open', async () => {
        const issues$ = new Subject<Issue[]>();
        const { element, view, load } = await renderIssuePicker(issues$);

        view.dispatch({ changes: { from: 9, to: 9, insert: '#' }, selection: { anchor: 10 } });
        await openPicker(view);
        expect(load).toHaveBeenCalledWith({
            idProject: 7,
            orderColumn: 'createAt',
            orderDirection: 'desc'
        });
        expect(tooltipOptions(element)).toEqual([]);

        issues$.next([issueOption(123, 'Fix login'), issueOption(456, 'Add dark mode')]);
        await openPicker(view);
        expect(tooltipOptions(element)).toEqual(['123', '456']);

        issues$.next([issueOption(123, 'Fix login'), issueOption(456, 'Add dark mode')]);
        closeCompletion(view);
        view.dispatch({ changes: { from: 10, to: 10, insert: '4' }, selection: { anchor: 11 } });
        await openPicker(view);
        expect(load).toHaveBeenCalledTimes(3);
        expect(tooltipOptions(element)).toEqual(['456']);
    });

    it('keeps offering the cached list while a refresh is in flight', async () => {
        const issues$ = new Subject<Issue[]>();
        const { element, view, load } = await renderIssuePicker(issues$);

        view.dispatch({ changes: { from: 9, to: 9, insert: '#' }, selection: { anchor: 10 } });
        await openPicker(view);
        issues$.next([issueOption(123, 'Fix login')]);

        await openPicker(view);
        expect(load).toHaveBeenCalledTimes(2);
        expect(tooltipOptions(element)).toEqual(['123']);
    });
});

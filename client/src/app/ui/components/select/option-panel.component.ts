import {
    ChangeDetectionStrategy,
    Component,
    ElementRef,
    TemplateRef,
    ViewEncapsulation,
    afterRenderEffect,
    input,
    output,
    viewChildren
} from '@angular/core';

/**
 * Presentational option list shared by the ui-select family. Renders the
 * (already-filtered) `visibleOptions`, marks the highlighted + selected rows,
 * projects an optional item template, and shows an empty message. Owns the
 * scroll-into-view of the highlighted row (`UiOptionNav` holds no DOM).
 *
 * Purely derivational: no selection/value/overlay/keyboard state — the shell
 * feeds inputs and handles `pick`/`highlightRequest`.
 */
@Component({
    selector: 'ui-option-panel',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    templateUrl: './option-panel.component.html'
})
export class UiOptionPanelComponent<T> {
    public readonly visibleOptions = input.required<readonly T[]>();
    public readonly getOptionLabel = input.required<(option: T) => string>();
    public readonly isOptionSelected = input<(option: T) => boolean>(() => false);
    public readonly highlightedIndex = input<number>(-1);
    public readonly itemTemplate = input<TemplateRef<{ $implicit: T }> | undefined>();
    public readonly actionsTemplate = input<
        TemplateRef<{ $implicit: T; openDock: (kind: string) => void }> | undefined
    >();
    public readonly openDock = input<(option: T, kind: string) => void>(() => {});
    public readonly emptyMessage = input<string>();
    public readonly emptyFilterMessage = input<string>();
    public readonly isFiltered = input<boolean>(false);
    public readonly showCheckbox = input<boolean>(false);
    public readonly listId = input<string>();
    public readonly optionIdPrefix = input<string>();
    /** 'presentation' when an ancestor already owns role=listbox (inline ui-listbox). */
    public readonly listRole = input<'listbox' | 'presentation'>('listbox');
    public readonly multiselectable = input<boolean>(false);

    /** A row was chosen (click). */
    public readonly pick = output<T>();
    /** Pointer entered a row → shell should sync the highlight index. */
    public readonly highlightRequest = output<number>();

    private readonly optionEls = viewChildren<ElementRef<HTMLLIElement>>('optionEl');

    protected bindOpenDock(option: T): (kind: string) => void {
        return (kind: string) => this.openDock()(option, kind);
    }

    public constructor() {
        // Scroll the highlighted row into view whenever the index changes.
        afterRenderEffect(() => {
            const index = this.highlightedIndex();
            if (index >= 0) {
                this.optionEls()[index]?.nativeElement.scrollIntoView({ block: 'nearest' });
            }
        });
    }
}

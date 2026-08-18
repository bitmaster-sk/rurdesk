import {
    ChangeDetectionStrategy,
    Component,
    TemplateRef,
    ViewEncapsulation,
    computed,
    contentChild,
    input,
    output
} from '@angular/core';
import { UiOptionNav } from './option-nav';

import { OptionConverter } from '../../converter/option.converter';

type OptionRecord = Record<string, unknown>;

/**
 * Inline, always-open option list used as the "list half" of a hand-rolled
 * picker inside a popover. Action
 * mode: emits `onChange` on every pick (no value binding, no CVA, no toggle-to-
 * null). Shares `UiOptionPanel` + `UiOptionNav`; reuses `.ui-select-*` chrome.
 */
@Component({
    selector: 'ui-listbox',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    template: `
        <div
            class="ui-listbox"
            tabindex="0"
            role="listbox"
            [attr.aria-activedescendant]="activeDescendantId()"
            (keydown)="onKeydown($event)"
        >
            @if (filter()) {
                <div class="ui-select-panel__filter-wrap">
                    <input
                        #filterInput
                        class="ui-input ui-select-panel__filter"
                        type="text"
                        [placeholder]="filterPlaceholder() || ''"
                        (input)="nav.setFilter(filterInput.value)"
                    />
                </div>
            }
            <ui-option-panel
                [visibleOptions]="nav.visibleOptions()"
                [getOptionLabel]="boundGetOptionLabel"
                [listRole]="'presentation'"
                [highlightedIndex]="nav.highlightedIndex()"
                [itemTemplate]="itemTpl()"
                [emptyMessage]="emptyMessage()"
                [emptyFilterMessage]="emptyFilterMessage()"
                [isFiltered]="!!nav.filterText()"
                [listId]="baseId + '_list'"
                [optionIdPrefix]="baseId"
                (pick)="pick($event)"
                (highlightRequest)="nav.setHighlight($event)"
            />
        </div>
    `
})
export class UiListboxComponent<T> {
    public readonly options = input.required<readonly T[]>();
    public readonly optionLabel = input<string>();
    public readonly filter = input(false);
    public readonly filterPlaceholder = input<string>();
    public readonly emptyMessage = input<string>();
    public readonly emptyFilterMessage = input<string>();

    /** Action emit — fires on every pick (no value binding). */
    public readonly valueChanged = output<{ originalEvent?: Event; value: T }>();

    protected readonly itemTpl = contentChild<TemplateRef<{ $implicit: T }>>('item');

    protected readonly baseId = `ui-listbox-${nextId()}`;

    protected readonly nav = new UiOptionNav<T>(
        this.options,
        option => this.getOptionLabel(option),
        () => -1
    );

    protected readonly activeDescendantId = computed(() => {
        const index = this.nav.highlightedIndex();
        return index >= 0 ? `${this.baseId}_opt_${index}` : null;
    });

    protected readonly boundGetOptionLabel = (option: T): string => this.getOptionLabel(option);

    protected pick(option: T, originalEvent?: Event): void {
        this.valueChanged.emit({ originalEvent, value: option });
    }

    protected onKeydown(event: KeyboardEvent): void {
        switch (event.key) {
            case 'ArrowDown':
                event.preventDefault();
                this.nav.moveHighlight(1);
                break;
            case 'ArrowUp':
                event.preventDefault();
                this.nav.moveHighlight(-1);
                break;
            case 'Home':
                event.preventDefault();
                this.nav.first();
                break;
            case 'End':
                event.preventDefault();
                this.nav.last();
                break;
            case 'Enter':
            case ' ': {
                event.preventDefault();
                const option = this.nav.visibleOptions()[this.nav.highlightedIndex()];
                if (option !== undefined) {
                    this.pick(option, event);
                }
                break;
            }
            default:
                if (
                    !this.filter() &&
                    event.key.length === 1 &&
                    !event.ctrlKey &&
                    !event.metaKey &&
                    !event.altKey
                ) {
                    this.nav.typeAhead(event.key);
                }
        }
    }

    protected getOptionLabel(option: T): string {
        const key = this.optionLabel();
        if (key) {
            return OptionConverter.toLabel((option as OptionRecord)[key]);
        }
        const label = (option as OptionRecord)?.['label'];
        return OptionConverter.toLabel(label ?? option);
    }
}

let idCounter = 0;
function nextId(): number {
    return ++idCounter;
}

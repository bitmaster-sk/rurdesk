import { Directive, computed, inject, input } from '@angular/core';
import { UiTableSortDirective } from './table-sort.directive';

/**
 * Sortable header for `.ui-table`. Placed on `<th uiSortColumn="field">`; injects
 * the table-level `[uiTableSort]` to read/toggle state.
 *
 * Exposes `icon()` (tabler icon name) via `exportAs` so the header renders its own
 * indicator: `<th uiSortColumn="x" #s="uiSortColumn"><tabler-icon [icon]="s.icon()"/></th>`.
 * No separate `field` input on the icon — it reads this directive directly.
 *
 * Emits proper `aria-sort` tokens (`ascending`/`descending`/`none`) rather than the
 * raw order value. `role="columnheader"` is omitted — redundant on `<th>`.
 */
@Directive({
    selector: 'th[uiSortColumn]',
    exportAs: 'uiSortColumn',
    standalone: false,
    host: {
        'class': 'ui-sort-column',
        'tabindex': '0',
        '[class.ui-sort-column--active]': 'order() !== 0',
        '[attr.aria-sort]': 'ariaSort()',
        '(click)': 'toggle()',
        '(keydown.enter)': 'onKey($event)',
        '(keydown.space)': 'onKey($event)'
    }
})
export class UiSortColumnDirective {
    public readonly field = input.required<string>({ alias: 'uiSortColumn' });

    private readonly table = inject(UiTableSortDirective);

    protected readonly order = computed(() => this.table.orderFor(this.field()));

    /** Tabler icon name reflecting current sort state (read by the header template). */
    public readonly icon = computed(() => {
        const o = this.order();
        return o === 1 ? 'sort-ascending' : o === -1 ? 'sort-descending' : 'arrows-sort';
    });

    protected readonly ariaSort = computed(() => {
        const o = this.order();
        return o === 1 ? 'ascending' : o === -1 ? 'descending' : 'none';
    });

    protected toggle(): void {
        this.table.toggle(this.field());
    }

    protected onKey(event: Event): void {
        event.preventDefault();
        this.toggle();
    }
}

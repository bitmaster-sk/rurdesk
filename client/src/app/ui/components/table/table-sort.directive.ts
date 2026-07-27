import { Directive, input, output, signal } from '@angular/core';

/** Emitted on user-driven sort change. `sortOrder`: 1 asc / -1 desc. */
export interface UiTableSortEvent {
    sortField: string;
    sortOrder: 1 | -1;
}

/**
 * Sort coordinator for the `.ui-table` family. Placed on `<table uiTableSort>`;
 * child `[uiSortColumn]` headers inject it to read/toggle state. Server-side
 * sort only — this holds state and emits, it never sorts the array.
 *
 * Binary toggle asc↔desc; switching to a new column starts at asc (1). The
 * initial `sortField`/`sortOrder` seed the displayed state WITHOUT emitting —
 * only a click/keyboard interaction emits `sortChange`.
 */
@Directive({
    selector: 'table[uiTableSort]',
    standalone: false
})
export class UiTableSortDirective {
    /** Initial sorted column (display only; does not emit). */
    public readonly sortField = input<string | null>(null);
    /** Initial sort direction (1 asc / -1 desc). */
    public readonly sortOrder = input<1 | -1>(1);

    public readonly sortChange = output<UiTableSortEvent>();

    /** User-driven state; null until first interaction, then overrides the seed. */
    private readonly state = signal<{ field: string; order: 1 | -1 } | null>(null);

    /** Active direction for a column, or 0 when it is not the sorted column. */
    public orderFor(field: string): 1 | -1 | 0 {
        const active =
            this.state() ??
            (this.sortField() ? { field: this.sortField()!, order: this.sortOrder() } : null);
        return active && active.field === field ? active.order : 0;
    }

    /** Toggle sort on a column and emit. Binary: same column flips, new column → asc. */
    public toggle(field: string): void {
        const current =
            this.state() ??
            (this.sortField() ? { field: this.sortField()!, order: this.sortOrder() } : null);
        const order: 1 | -1 =
            current && current.field === field ? (current.order === 1 ? -1 : 1) : 1;
        this.state.set({ field, order });
        this.sortChange.emit({ sortField: field, sortOrder: order });
    }
}

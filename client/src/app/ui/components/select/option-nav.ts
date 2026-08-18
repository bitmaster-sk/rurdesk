import { Signal, computed, signal } from '@angular/core';

/**
 * Shared navigation/filter engine for the ui-select family
 * (`ui-select` / `ui-multiselect` / `ui-listbox`).
 *
 * A plain, per-control signal class (NOT an Angular service — state is
 * per-instance) that owns ONLY the option-list logic: the filter text, the
 * derived visible-options list, the highlighted INDEX, and type-ahead. It holds
 * no DOM — scroll-into-view lives in `UiOptionPanelComponent` — so it is fully
 * unit-testable without a browser.
 *
 * The consuming shell wires keyboard events to the discrete methods here and
 * decides open/close/select; the panel renders `visibleOptions` + `highlightedIndex`.
 */
export class UiOptionNav<T> {
    /** Current filter query (empty when the shell has no filter box). */
    public readonly filterText = signal('');

    /** Index into `visibleOptions` of the highlighted row; -1 = none. */
    public readonly highlightedIndex = signal(-1);

    /** Options after applying the filter query. Single source for all shells. */
    public readonly visibleOptions = computed<readonly T[]>(() => {
        const query = this.filterText().trim().toLowerCase();
        const options = this.options();
        if (!query) {
            return options;
        }
        return options.filter(option => this.getLabel(option).toLowerCase().includes(query));
    });

    private typeAheadBuffer = '';
    private typeAheadAt = 0;

    private readonly options: Signal<readonly T[]>;
    private readonly getLabel: (option: T) => string;
    /** Index of the selected option in `visibleOptions` (single-select), or -1
     *  (multiselect/listbox have no single selected row → land on 0 on open). */
    private readonly getSelectedIndex: () => number;
    private readonly now: () => number;

    public constructor(
        options: Signal<readonly T[]>,
        getLabel: (option: T) => string,
        getSelectedIndex: () => number,
        now: () => number = () => performance.now()
    ) {
        this.options = options;
        this.getLabel = getLabel;
        this.getSelectedIndex = getSelectedIndex;
        this.now = now;
    }

    /** On open: highlight the selected option, else the first option. */
    public initHighlight(): void {
        const selected = this.getSelectedIndex();
        const length = this.visibleOptions().length;
        this.highlightedIndex.set(selected >= 0 ? selected : length ? 0 : -1);
    }

    /** Move the highlight by `delta`, clamped to the visible list. */
    public moveHighlight(delta: number): void {
        const length = this.visibleOptions().length;
        if (!length) {
            this.highlightedIndex.set(-1);
            return;
        }
        const current = this.highlightedIndex();
        if (current < 0) {
            this.highlightedIndex.set(delta > 0 ? 0 : length - 1);
            return;
        }
        this.highlightedIndex.set(Math.min(length - 1, Math.max(0, current + delta)));
    }

    public setHighlight(index: number): void {
        this.highlightedIndex.set(index);
    }

    public first(): void {
        this.highlightedIndex.set(this.visibleOptions().length ? 0 : -1);
    }

    public last(): void {
        const length = this.visibleOptions().length;
        this.highlightedIndex.set(length ? length - 1 : -1);
    }

    /** Update the filter query and reset the highlight to the first match. */
    public setFilter(text: string): void {
        this.filterText.set(text);
        this.highlightedIndex.set(this.visibleOptions().length ? 0 : -1);
    }

    /** Type-to-search over the visible options (used only when the filter box is off). */
    public typeAhead(char: string): void {
        const at = this.now();
        this.typeAheadBuffer = at - this.typeAheadAt > 500 ? char : this.typeAheadBuffer + char;
        this.typeAheadAt = at;
        const query = this.typeAheadBuffer.toLowerCase();
        const index = this.visibleOptions().findIndex(option =>
            this.getLabel(option).toLowerCase().startsWith(query)
        );
        if (index >= 0) {
            this.highlightedIndex.set(index);
        }
    }

    /** Clear filter + highlight + type-ahead (on close). */
    public reset(): void {
        this.filterText.set('');
        this.highlightedIndex.set(-1);
        this.typeAheadBuffer = '';
        this.typeAheadAt = 0;
    }
}

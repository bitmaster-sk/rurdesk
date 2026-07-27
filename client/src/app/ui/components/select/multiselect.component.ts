import { Overlay, OverlayRef } from '@angular/cdk/overlay';
import { TemplatePortal } from '@angular/cdk/portal';
import {
    ChangeDetectionStrategy,
    Component,
    ElementRef,
    OnDestroy,
    TemplateRef,
    ViewContainerRef,
    ViewEncapsulation,
    computed,
    contentChild,
    forwardRef,
    inject,
    input,
    output,
    signal,
    viewChild
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import isEqual from 'lodash-es/isEqual';
import { UiOptionNav } from './option-nav';

type OptionRecord = Record<string, unknown>;

/**
 * Multi-select dropdown, scoped to app usage. Array `ControlValueAccessor`;
 * toggling an option keeps the panel open. Trigger shows a comma-joined label
 * by default or a projected `#selectedItems` template. Keeps the select-all
 * header (tri-state). Shares `UiOptionPanel` + `UiOptionNav` with `ui-select`;
 * reuses `.ui-select-*`.
 */
@Component({
    selector: 'ui-multiselect',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => UiMultiSelectComponent),
            multi: true
        }
    ],
    template: `
        <div
            #trigger
            class="ui-select-trigger"
            role="combobox"
            aria-haspopup="listbox"
            [id]="inputId()"
            [class.ui-select-trigger--sm]="size() === 'small'"
            [class.ui-select-trigger--lg]="size() === 'large'"
            [class.ui-select-trigger--disabled]="isDisabled()"
            [class.ui-select-trigger--open]="isOpen()"
            [attr.tabindex]="isDisabled() ? -1 : 0"
            [attr.aria-expanded]="isOpen()"
            [attr.aria-controls]="isOpen() ? baseId + '_list' : null"
            [attr.aria-activedescendant]="activeDescendantId()"
            [attr.aria-disabled]="isDisabled()"
            (click)="onTriggerClick()"
            (keydown)="onKeydown($event)"
            (blur)="onTouched()"
        >
            <span class="ui-select-trigger__value">
                @if (selectedOptions().length) {
                    @if (selectedItemsTpl()) {
                        <ng-container
                            [ngTemplateOutlet]="selectedItemsTpl()!"
                            [ngTemplateOutletContext]="{ $implicit: selectedOptions() }"
                        />
                    } @else {
                        {{ commaLabel() }}
                    }
                } @else {
                    <span class="ui-select-trigger__placeholder">{{ placeholder() }}</span>
                }
            </span>
            <tabler-icon
                class="ui-select-trigger__chevron"
                icon="chevron-down"
                [size]="chevronSize()"
            />
        </div>

        <ng-template #panelTpl>
            <!-- stopPropagation: keep a surrounding <ui-popover> open when
                 clicking options in this body-level CDK overlay. See ui-select. -->
            <div class="ui-select-panel" (pointerdown)="$event.stopPropagation()">
                @if (showToggleAll()) {
                    <div class="ui-select-panel__header">
                        <span
                            class="ui-select-panel__checkbox"
                            [class.ui-select-panel__checkbox--checked]="toggleAllState() === 'all'"
                            [class.ui-select-panel__checkbox--indeterminate]="
                                toggleAllState() === 'some'
                            "
                            role="checkbox"
                            [attr.aria-checked]="
                                toggleAllState() === 'all'
                                    ? 'true'
                                    : toggleAllState() === 'some'
                                      ? 'mixed'
                                      : 'false'
                            "
                            tabindex="-1"
                            (mousedown)="$event.preventDefault()"
                            (click)="toggleAll()"
                        ></span>
                    </div>
                }
                <ui-option-panel
                    [visibleOptions]="nav.visibleOptions()"
                    [getOptionLabel]="boundGetOptionLabel"
                    [isOptionSelected]="boundIsOptionSelected"
                    [showCheckbox]="true"
                    [multiselectable]="true"
                    [highlightedIndex]="nav.highlightedIndex()"
                    [itemTemplate]="itemTpl()"
                    [emptyMessage]="emptyMessage()"
                    [emptyFilterMessage]="emptyFilterMessage()"
                    [isFiltered]="!!nav.filterText()"
                    [listId]="baseId + '_list'"
                    [optionIdPrefix]="baseId"
                    (pick)="toggleOption($event)"
                    (highlightRequest)="nav.setHighlight($event)"
                />
            </div>
        </ng-template>
    `
})
export class UiMultiSelectComponent<T> implements ControlValueAccessor, OnDestroy {
    public readonly options = input.required<readonly T[]>();
    public readonly optionLabel = input<string>();
    public readonly optionValue = input<string>();
    public readonly placeholder = input<string>();
    public readonly size = input<'small' | 'large' | undefined>();

    /** Chevron scales with the tier (14 / 16 / 18) — see UiSelectComponent. */
    protected readonly chevronSize = computed(() =>
        this.size() === 'small' ? 14 : this.size() === 'large' ? 18 : 16
    );
    public readonly emptyMessage = input<string>();
    public readonly emptyFilterMessage = input<string>();
    public readonly inputId = input<string>();
    public readonly showToggleAll = input(true);

    public readonly onChange = output<{ originalEvent?: Event; value: T[]; itemValue?: T }>();

    private readonly overlay = inject(Overlay);
    private readonly vcr = inject(ViewContainerRef);

    private readonly trigger = viewChild.required<ElementRef<HTMLElement>>('trigger');
    private readonly panelTpl = viewChild.required<TemplateRef<unknown>>('panelTpl');
    protected readonly selectedItemsTpl =
        contentChild<TemplateRef<{ $implicit: T[] }>>('selectedItems');
    protected readonly itemTpl = contentChild<TemplateRef<{ $implicit: T }>>('item');

    /** CVA model value = array of resolved option values. */
    private readonly value = signal<unknown[]>([]);
    protected readonly isDisabled = signal(false);
    protected readonly isOpen = signal(false);

    protected readonly baseId = `ui-multiselect-${nextId()}`;

    protected readonly nav = new UiOptionNav<T>(
        this.options,
        option => this.getOptionLabel(option),
        () => -1
    );

    protected readonly selectedOptions = computed<T[]>(() => {
        const value = this.value();
        return this.options().filter(option =>
            value.some(v => isEqual(v, this.getOptionValue(option)))
        );
    });

    protected readonly commaLabel = computed(() =>
        this.selectedOptions()
            .map(option => this.getOptionLabel(option))
            .join(', ')
    );

    protected readonly toggleAllState = computed<'all' | 'some' | 'none'>(() => {
        const selected = this.value().length;
        const total = this.options().length;
        if (selected > 0 && selected === total) {
            return 'all';
        }
        return selected > 0 ? 'some' : 'none';
    });

    protected readonly activeDescendantId = computed(() => {
        const index = this.nav.highlightedIndex();
        return this.isOpen() && index >= 0 ? `${this.baseId}_opt_${index}` : null;
    });

    protected readonly boundGetOptionLabel = (option: T): string => this.getOptionLabel(option);
    protected readonly boundIsOptionSelected = (option: T): boolean =>
        this.value().some(v => isEqual(v, this.getOptionValue(option)));

    private overlayRef: OverlayRef | null = null;
    private onChangeFn: (value: unknown[]) => void = () => {};
    protected onTouched: () => void = () => {};

    // ─── ControlValueAccessor ────────────────────────────────────────────────
    public ngOnDestroy(): void {
        // Dispose directly (not close(), which refocuses a now-gone trigger): a
        // component destroyed while open would orphan the CDK overlay pane in the
        // DOM — invisible but pointer-events:auto, eating later clicks.
        this.overlayRef?.dispose();
        this.overlayRef = null;
    }

    public writeValue(value: unknown[] | null): void {
        this.value.set(value ?? []);
    }
    public registerOnChange(fn: (value: unknown[]) => void): void {
        this.onChangeFn = fn;
    }
    public registerOnTouched(fn: () => void): void {
        this.onTouched = fn;
    }
    public setDisabledState(isDisabled: boolean): void {
        this.isDisabled.set(isDisabled);
        if (isDisabled) {
            this.close();
        }
    }

    // ─── Interaction ─────────────────────────────────────────────────────────
    protected onTriggerClick(): void {
        if (this.isDisabled()) {
            return;
        }
        this.isOpen() ? this.close() : this.open();
    }

    protected toggleOption(option: T, originalEvent?: Event): void {
        const optionValue = this.getOptionValue(option);
        const current = this.value();
        const isSelected = current.some(v => isEqual(v, optionValue));
        const next = isSelected
            ? current.filter(v => !isEqual(v, optionValue))
            : [...current, optionValue];
        this.commit(next, option, originalEvent);
    }

    protected toggleAll(): void {
        const next =
            this.toggleAllState() === 'all'
                ? []
                : this.options().map(option => this.getOptionValue(option));
        this.commit(next);
    }

    private commit(next: unknown[], itemValue?: T, originalEvent?: Event): void {
        this.value.set(next);
        this.onChangeFn(next);
        this.onChange.emit({ originalEvent, value: next as T[], itemValue });
    }

    protected onKeydown(event: KeyboardEvent): void {
        switch (event.key) {
            case 'ArrowDown':
                event.preventDefault();
                this.isOpen() ? this.nav.moveHighlight(1) : this.open();
                break;
            case 'ArrowUp':
                event.preventDefault();
                this.isOpen() ? this.nav.moveHighlight(-1) : this.open();
                break;
            case 'Home':
                if (this.isOpen()) {
                    event.preventDefault();
                    this.nav.first();
                }
                break;
            case 'End':
                if (this.isOpen()) {
                    event.preventDefault();
                    this.nav.last();
                }
                break;
            case 'Enter':
            case ' ':
                event.preventDefault();
                if (this.isOpen()) {
                    this.toggleHighlighted(event);
                } else {
                    this.open();
                }
                break;
            case 'Escape':
                if (this.isOpen()) {
                    event.preventDefault();
                    this.close();
                }
                break;
            case 'Tab':
                if (this.isOpen()) {
                    this.close();
                }
                break;
            default:
                if (event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) {
                    this.nav.typeAhead(event.key);
                }
        }
    }

    private toggleHighlighted(event: Event): void {
        const option = this.nav.visibleOptions()[this.nav.highlightedIndex()];
        if (option !== undefined) {
            this.toggleOption(option, event); // stays open (parity)
        }
    }

    // ─── Overlay ─────────────────────────────────────────────────────────────
    private open(): void {
        if (this.isDisabled() || this.isOpen() || this.overlayRef) {
            return;
        }
        const triggerEl = this.trigger().nativeElement;
        const positionStrategy = this.overlay
            .position()
            .flexibleConnectedTo(triggerEl)
            .withFlexibleDimensions(false)
            .withPush(false)
            .withPositions([
                {
                    originX: 'start',
                    originY: 'bottom',
                    overlayX: 'start',
                    overlayY: 'top',
                    offsetY: 4
                },
                {
                    originX: 'start',
                    originY: 'top',
                    overlayX: 'start',
                    overlayY: 'bottom',
                    offsetY: -4
                }
            ]);

        this.overlayRef = this.overlay.create({
            positionStrategy,
            scrollStrategy: this.overlay.scrollStrategies.reposition(),
            // minWidth (not width): grow to fit longer option labels.
            minWidth: triggerEl.offsetWidth,
            hasBackdrop: false
        });
        this.overlayRef.attach(new TemplatePortal(this.panelTpl(), this.vcr));
        this.overlayRef.outsidePointerEvents().subscribe(event => {
            // Ignore pointer events on the trigger: its own (click) toggles. Without
            // this the pointerdown closes here and the following click reopens (flaky).
            if (this.trigger().nativeElement.contains(event.target as Node)) {
                return;
            }
            this.close(false); // outside click: don't yank focus back to the trigger
        });

        this.isOpen.set(true);
        this.nav.initHighlight();
    }

    private close(restoreFocus = true): void {
        if (!this.overlayRef) {
            return;
        }
        this.overlayRef.dispose();
        this.overlayRef = null;
        this.isOpen.set(false);
        this.nav.reset();
        this.onTouched();
        if (restoreFocus) {
            this.trigger().nativeElement.focus();
        }
    }

    // ─── Option resolution ───────────────────────────────────────────────────
    protected getOptionLabel(option: T): string {
        const key = this.optionLabel();
        if (key) {
            return String((option as OptionRecord)[key] ?? '');
        }
        const label = (option as OptionRecord)?.['label'];
        return String(label ?? option ?? '');
    }

    private getOptionValue(option: T): unknown {
        const key = this.optionValue();
        return key ? (option as OptionRecord)[key] : option;
    }
}

let idCounter = 0;
function nextId(): number {
    return ++idCounter;
}

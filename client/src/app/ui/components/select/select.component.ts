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
 * Single-select dropdown, scoped to the features this app uses. A real
 * `ControlValueAccessor` with a CDK-overlay panel (`TemplatePortal`, following
 * `confirm.directive.ts`) over a shared `UiOptionPanel` + `UiOptionNav` engine.
 *
 * Closed trigger reuses the `--ui-input-*` chrome via `.ui-select-trigger`.
 * Escape/keyboard are owned here (the CDK `keydownEvents()` subscription is
 * deliberately NOT used — it would double-fire with the trigger handler).
 */
@Component({
    selector: 'ui-select',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => UiSelectComponent),
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
            (keydown)="onKeydown($event, false)"
            (blur)="onTouched()"
        >
            <span class="ui-select-trigger__value">
                @if (selectedOption(); as option) {
                    @if (selectedItemTpl()) {
                        <ng-container
                            [ngTemplateOutlet]="selectedItemTpl()!"
                            [ngTemplateOutletContext]="{ $implicit: option }"
                        />
                    } @else {
                        {{ getOptionLabel(option) }}
                    }
                } @else {
                    <span class="ui-select-trigger__placeholder">{{ placeholder() }}</span>
                }
            </span>

            @if (showClear() && selectedOption()) {
                <button
                    type="button"
                    class="ui-select-trigger__clear"
                    [attr.aria-label]="'UI.CLEAR' | translate"
                    (click)="onClear($event)"
                >
                    <tabler-icon icon="x" [size]="14" />
                </button>
            }
            <tabler-icon
                class="ui-select-trigger__chevron"
                icon="chevron-down"
                [size]="chevronSize()"
            />
        </div>

        <ng-template #panelTpl>
            <!-- stopPropagation: the panel renders in a body-level CDK overlay,
                 physically outside any surrounding <ui-popover>. Without this,
                 a pointerdown on an option counts as an "outside" click for that
                 popover and closes it before the pick registers (z-index ≠ dismissal).
                 Harmless for non-nested selects (CDK's own outside detection is unaffected). -->
            <div class="ui-select-panel" (pointerdown)="$event.stopPropagation()">
                @if (filter()) {
                    <div class="ui-select-panel__filter-wrap">
                        <input
                            #filterInput
                            class="ui-input ui-select-panel__filter"
                            type="text"
                            [placeholder]="filterPlaceholder() || ''"
                            [attr.aria-controls]="baseId + '_list'"
                            [attr.aria-activedescendant]="activeDescendantId()"
                            (input)="nav.setFilter(filterInput.value)"
                            (keydown)="onKeydown($event, true)"
                        />
                    </div>
                }
                <ui-option-panel
                    [visibleOptions]="nav.visibleOptions()"
                    [getOptionLabel]="boundGetOptionLabel"
                    [isOptionSelected]="boundIsOptionSelected"
                    [highlightedIndex]="nav.highlightedIndex()"
                    [itemTemplate]="itemTpl()"
                    [emptyMessage]="emptyMessage()"
                    [emptyFilterMessage]="emptyFilterMessage()"
                    [isFiltered]="!!nav.filterText()"
                    [listId]="baseId + '_list'"
                    [optionIdPrefix]="baseId"
                    (pick)="selectOption($event)"
                    (highlightRequest)="nav.setHighlight($event)"
                />
            </div>
        </ng-template>
    `
})
export class UiSelectComponent<T> implements ControlValueAccessor, OnDestroy {
    public readonly options = input.required<readonly T[]>();
    public readonly optionLabel = input<string>();
    public readonly optionValue = input<string>();
    public readonly placeholder = input<string>();
    public readonly filter = input(false);
    public readonly filterPlaceholder = input<string>();
    public readonly showClear = input(false);
    public readonly size = input<'small' | 'large' | undefined>();

    /** Chevron scales with the tier so an empty trigger's height stays under the
     *  text line-box and matches its input at every size (14 / 16 / 18). */
    protected readonly chevronSize = computed(() =>
        this.size() === 'small' ? 14 : this.size() === 'large' ? 18 : 16
    );
    public readonly emptyMessage = input<string>();
    public readonly emptyFilterMessage = input<string>();
    public readonly inputId = input<string>();
    /** Component-level disabled (e.g. project-members `[disabled]="isLastOwner(user)"`).
     *  Combined with the forms `setDisabledState` below into `isDisabled`. */
    public readonly disabled = input(false);

    /** User-driven change only (never on writeValue) — e.g. top-menu navigation.
     *  `value` is the resolved model value (optionValue when set), whose type is
     *  dynamic — hence `unknown` (consumers narrow). */
    public readonly onChange = output<{ originalEvent?: Event; value: unknown }>();

    private readonly overlay = inject(Overlay);
    private readonly vcr = inject(ViewContainerRef);

    private readonly trigger = viewChild.required<ElementRef<HTMLElement>>('trigger');
    private readonly panelTpl = viewChild.required<TemplateRef<unknown>>('panelTpl');
    protected readonly selectedItemTpl =
        contentChild<TemplateRef<{ $implicit: T }>>('selectedItem');
    protected readonly itemTpl = contentChild<TemplateRef<{ $implicit: T }>>('item');

    /** CVA model value (resolved option value, i.e. optionValue or whole option). */
    private readonly value = signal<unknown>(null);
    private readonly disabledFromForm = signal(false);
    protected readonly isDisabled = computed(() => this.disabled() || this.disabledFromForm());
    protected readonly isOpen = signal(false);

    /** Stable per-instance id base for ARIA wiring across the overlay boundary. */
    protected readonly baseId = `ui-select-${nextId()}`;

    protected readonly nav = new UiOptionNav<T>(
        this.options,
        option => this.getOptionLabel(option),
        () => this.selectedIndexInVisible()
    );

    protected readonly selectedOption = computed<T | null>(() => {
        const value = this.value();
        return this.options().find(option => isEqual(this.getOptionValue(option), value)) ?? null;
    });

    protected readonly activeDescendantId = computed(() => {
        const index = this.nav.highlightedIndex();
        return this.isOpen() && index >= 0 ? `${this.baseId}_opt_${index}` : null;
    });

    // Stable bound callbacks passed as inputs to the presentational panel.
    protected readonly boundGetOptionLabel = (option: T): string => this.getOptionLabel(option);
    protected readonly boundIsOptionSelected = (option: T): boolean =>
        isEqual(this.getOptionValue(option), this.value());

    private overlayRef: OverlayRef | null = null;
    private onChangeFn: (value: unknown) => void = () => {};
    protected onTouched: () => void = () => {};

    public ngOnDestroy(): void {
        // Dispose directly (not close(), which refocuses a now-gone trigger): a
        // component destroyed while open would otherwise orphan the CDK overlay
        // pane in the DOM — invisible but pointer-events:auto, eating later clicks.
        this.overlayRef?.dispose();
        this.overlayRef = null;
    }

    // ─── ControlValueAccessor ────────────────────────────────────────────────
    public writeValue(value: unknown): void {
        this.value.set(value ?? null); // store only — label is derived; do NOT emit onChange
    }
    public registerOnChange(fn: (value: unknown) => void): void {
        this.onChangeFn = fn;
    }
    public registerOnTouched(fn: () => void): void {
        this.onTouched = fn;
    }
    public setDisabledState(isDisabled: boolean): void {
        this.disabledFromForm.set(isDisabled);
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

    protected selectOption(option: T, originalEvent?: Event): void {
        const value = this.getOptionValue(option);
        this.value.set(value);
        this.onChangeFn(value);
        this.onChange.emit({ originalEvent, value });
        this.close();
    }

    protected onClear(event: Event): void {
        event.stopPropagation();
        this.value.set(null);
        this.onChangeFn(null);
        this.onChange.emit({ originalEvent: event, value: null });
    }

    protected onKeydown(event: KeyboardEvent, fromFilter: boolean): void {
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
                event.preventDefault();
                if (this.isOpen()) {
                    this.pickHighlighted(event);
                } else {
                    this.open();
                }
                break;
            case ' ':
                if (!fromFilter) {
                    event.preventDefault();
                    this.isOpen() ? this.pickHighlighted(event) : this.open();
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
                if (
                    !fromFilter &&
                    event.key.length === 1 &&
                    !event.ctrlKey &&
                    !event.metaKey &&
                    !event.altKey
                ) {
                    this.nav.typeAhead(event.key);
                }
        }
    }

    private pickHighlighted(event: Event): void {
        const option = this.nav.visibleOptions()[this.nav.highlightedIndex()];
        if (option !== undefined) {
            this.selectOption(option, event);
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
            // minWidth (not width): at least as wide as the trigger, but grows to
            // fit longer option labels instead of clipping them to trigger width.
            minWidth: triggerEl.offsetWidth,
            hasBackdrop: false
        });
        this.overlayRef.attach(new TemplatePortal(this.panelTpl(), this.vcr));
        // Escape/keyboard is owned by the trigger/filter handlers, NOT
        // overlayRef.keydownEvents() (that would double-fire).
        this.overlayRef.outsidePointerEvents().subscribe(event => {
            // Ignore pointer events on the trigger itself: its own (click) handler
            // toggles open/closed. Without this, the pointerdown closes here and the
            // following click reopens — the select reads as "won't close" (flaky).
            if (this.trigger().nativeElement.contains(event.target as Node)) {
                return;
            }
            this.close(false); // outside click: don't yank focus back to the trigger
        });

        this.isOpen.set(true);
        this.nav.initHighlight();

        if (this.filter()) {
            setTimeout(() => {
                this.overlayRef?.overlayElement
                    .querySelector<HTMLInputElement>('.ui-select-panel__filter')
                    ?.focus();
            });
        }
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

    private selectedIndexInVisible(): number {
        const selected = this.selectedOption();
        if (!selected) {
            return -1;
        }
        return this.nav.visibleOptions().indexOf(selected);
    }
}

let idCounter = 0;
function nextId(): number {
    return ++idCounter;
}

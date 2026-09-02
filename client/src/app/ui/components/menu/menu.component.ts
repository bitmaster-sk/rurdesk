import { Overlay, OverlayRef } from '@angular/cdk/overlay';
import { TemplatePortal } from '@angular/cdk/portal';
import {
    ChangeDetectionStrategy,
    Component,
    OnDestroy,
    TemplateRef,
    ViewContainerRef,
    ViewEncapsulation,
    contentChild,
    inject,
    input,
    signal,
    viewChild
} from '@angular/core';
import { UiMenuItem } from './menu-item.model';

/**
 * Model-driven popup menu. Toggled imperatively from a parent via a template
 * ref (`#ref`) exposing `toggle`/`hide`.
 *
 * The menu OWNS the interactive row element (`<a>`/`<button role="menuitem">`),
 * its focus, and its keyboard activation — a custom `#item` template fills only
 * the row interior (inner-only, like `ui-select`). This is what makes full
 * keyboard a11y (roving focus + Enter/Space) actually work.
 */
@Component({
    selector: 'ui-menu',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    template: `
        <ng-template #tpl>
            <ul class="ui-menu" [class]="panelClass()" role="menu">
                @for (item of model(); track trackItem(item)) {
                    @if (item.separator) {
                        <li class="ui-menu-separator" role="separator"></li>
                    } @else if (item.items) {
                        <!-- one-level group: header + flat sub-list; empty items = header only -->
                        <li role="presentation" class="ui-menu-group">
                            <span class="ui-menu-group-label" aria-hidden="true">
                                {{ item.labelKey ?? item.label | translate }}
                            </span>
                            <ul
                                class="ui-menu-group-list"
                                role="group"
                                [attr.aria-label]="item.labelKey ?? item.label | translate"
                            >
                                @for (sub of item.items; track trackItem(sub)) {
                                    <li role="none">
                                        <ng-container
                                            *ngTemplateOutlet="rowTpl; context: { $implicit: sub }"
                                        />
                                    </li>
                                }
                            </ul>
                        </li>
                    } @else {
                        <li role="none">
                            <ng-container
                                *ngTemplateOutlet="rowTpl; context: { $implicit: item }"
                            />
                        </li>
                    }
                }
            </ul>

            <!-- The menu owns the interactive element; #item fills only the interior. -->
            <ng-template #rowTpl let-item>
                @if (item.routerLink) {
                    <a
                        class="ui-menu-item"
                        role="menuitem"
                        tabindex="-1"
                        [routerLink]="item.routerLink"
                        (click)="onActivate(item)"
                    >
                        <ng-container
                            *ngTemplateOutlet="
                                itemTpl() ? itemTpl()! : defaultRow;
                                context: { $implicit: item }
                            "
                        />
                    </a>
                } @else {
                    <button
                        class="ui-menu-item"
                        type="button"
                        role="menuitem"
                        tabindex="-1"
                        (click)="onActivate(item)"
                    >
                        <ng-container
                            *ngTemplateOutlet="
                                itemTpl() ? itemTpl()! : defaultRow;
                                context: { $implicit: item }
                            "
                        />
                    </button>
                }
            </ng-template>

            <ng-template #defaultRow let-item>
                @if (item.icon) {
                    <tabler-icon class="ui-menu-item-icon" [icon]="item.icon" [size]="16" />
                }
                <span class="ui-menu-item-label">
                    {{ item.labelKey ?? item.label | translate }}
                </span>
                @if (item.badge) {
                    <ui-badge
                        class="ui-menu-item-badge"
                        [value]="item.badge"
                        [severity]="item.badgeSeverity ?? 'secondary'"
                    />
                }
            </ng-template>
        </ng-template>
    `
})
export class UiMenuComponent implements OnDestroy {
    /** Menu model (flat items and/or one-level groups). */
    public readonly model = input<UiMenuItem[]>([]);
    /** Extra class on both the overlay pane and the `<ul>` (so `.user-menu`-style targeting still hits). */
    public readonly panelClass = input<string>('');

    /** Optional custom row interior: `<ng-template #item let-item>`. */
    protected readonly itemTpl = contentChild<TemplateRef<{ $implicit: UiMenuItem }>>('item');

    private readonly overlay = inject(Overlay);
    private readonly vcr = inject(ViewContainerRef);
    private readonly tpl = viewChild.required<TemplateRef<unknown>>('tpl');

    protected readonly isOpen = signal(false);
    private overlayRef: OverlayRef | null = null;
    /** Anchor; excluded from outside-dismiss so its toggle handler governs closing. */
    private originEl: HTMLElement | null = null;
    /** Where focus returns on Escape/Tab/command activation (resolved at open). */
    private restoreFocusEl: HTMLElement | null = null;

    /** Stable track key: routerLink is unique+stable; command items key by label. */
    protected readonly trackItem = (item: UiMenuItem): unknown =>
        item.routerLink ? JSON.stringify(item.routerLink) : (item.label ?? item);

    public ngOnDestroy(): void {
        this.disposeOverlay();
    }

    /** Open anchored to the event's `currentTarget`, or close if already open. */
    public toggle(event: Event): void {
        if (this.isOpen()) {
            this.hide();
            return;
        }
        this.show(event);
    }

    /** Open anchored to an element (or the `currentTarget` of an event). */
    public show(target: HTMLElement | Event): void {
        if (this.isOpen()) {
            return;
        }
        const origin =
            target instanceof HTMLElement ? target : (target.currentTarget as HTMLElement | null);
        if (!origin) {
            return;
        }
        this.originEl = origin;
        this.restoreFocusEl = this.resolveRestoreFocus(origin);

        const positionStrategy = this.overlay
            .position()
            .flexibleConnectedTo(origin)
            .withViewportMargin(8)
            .withFlexibleDimensions(false)
            .withGrowAfterOpen(false)
            .withPush(true)
            .withPositions([
                {
                    originX: 'start',
                    originY: 'bottom',
                    overlayX: 'start',
                    overlayY: 'top',
                    offsetY: 6
                },
                {
                    originX: 'start',
                    originY: 'top',
                    overlayX: 'start',
                    overlayY: 'bottom',
                    offsetY: -6
                },
                { originX: 'end', originY: 'bottom', overlayX: 'end', overlayY: 'top', offsetY: 6 },
                { originX: 'end', originY: 'top', overlayX: 'end', overlayY: 'bottom', offsetY: -6 }
            ]);

        this.overlayRef = this.overlay.create({
            positionStrategy,
            scrollStrategy: this.overlay.scrollStrategies.reposition(),
            hasBackdrop: false,
            panelClass: this.panelClass() ? this.panelClass() : undefined
        });
        this.overlayRef.attach(new TemplatePortal(this.tpl(), this.vcr));
        this.overlayRef.outsidePointerEvents().subscribe(event => {
            const node = event.target as Node | null;
            const onOrigin = !!node && !!this.originEl && this.originEl.contains(node);
            if (!onOrigin) {
                this.hide();
            }
        });
        this.overlayRef.keydownEvents().subscribe(event => this.onKeydown(event));
        this.isOpen.set(true);

        // Focus the first item once the portal has rendered. The keyboard
        // dispatcher is document-level so Escape/arrows work even without this,
        // but focus-first is the expected menu UX.
        queueMicrotask(() => this.rows()[0]?.focus());
    }

    public hide(restoreFocus = false): void {
        if (!this.overlayRef) {
            return;
        }
        const restoreEl = this.restoreFocusEl;
        this.disposeOverlay();
        this.isOpen.set(false);
        if (restoreFocus) {
            restoreEl?.focus();
        }
    }

    protected onActivate(item: UiMenuItem): void {
        const hadCommand = !!item.command;
        item.command?.();
        // Command → return focus to the trigger; routerLink navigates away so don't.
        this.hide(hadCommand);
    }

    private onKeydown(event: KeyboardEvent): void {
        if (event.key === 'Escape') {
            event.preventDefault();
            this.hide(true);
            return;
        }
        if (event.key === 'Tab') {
            // Restore focus to the trigger, then let the default Tab advance from it.
            this.hide(true);
            return;
        }

        const rows = this.rows();
        if (rows.length === 0) {
            return;
        }
        const index = rows.indexOf(document.activeElement as HTMLElement);

        switch (event.key) {
            case 'ArrowDown':
                event.preventDefault();
                rows[index < rows.length - 1 ? index + 1 : 0].focus();
                break;
            case 'ArrowUp':
                event.preventDefault();
                rows[index > 0 ? index - 1 : rows.length - 1].focus();
                break;
            case 'Home':
                event.preventDefault();
                rows[0].focus();
                break;
            case 'End':
                event.preventDefault();
                rows[rows.length - 1].focus();
                break;
            case 'Enter':
            case ' ':
                if (index >= 0) {
                    // Single activation path for <a> and <button> alike: preventDefault
                    // stops anchor Space-scroll / button keyup-click, .click() drives both.
                    event.preventDefault();
                    rows[index].click();
                }
                break;
        }
    }

    /** Live, recursive row set (flattens nested groups) — recomputed each keydown. */
    private rows(): HTMLElement[] {
        const el = this.overlayRef?.overlayElement;
        return el ? Array.from(el.querySelectorAll<HTMLElement>('[role="menuitem"]')) : [];
    }

    /** The element focus returns to: the focused descendant of origin, origin
     *  itself if focusable, else its first focusable descendant, else origin. */
    private resolveRestoreFocus(origin: HTMLElement): HTMLElement {
        const active = document.activeElement;
        if (active instanceof HTMLElement && origin.contains(active)) {
            return active;
        }
        if (origin.matches?.('button, a[href], [tabindex]')) {
            return origin;
        }
        return origin.querySelector<HTMLElement>('button, a[href], [tabindex]') ?? origin;
    }

    private disposeOverlay(): void {
        this.overlayRef?.dispose();
        this.overlayRef = null;
        this.originEl = null;
    }
}

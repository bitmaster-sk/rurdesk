import { Overlay, OverlayRef } from '@angular/cdk/overlay';
import { TemplatePortal } from '@angular/cdk/portal';
import {
    ChangeDetectionStrategy,
    Component,
    OnDestroy,
    TemplateRef,
    ViewContainerRef,
    ViewEncapsulation,
    inject,
    input,
    output,
    signal,
    viewChild
} from '@angular/core';

/**
 * Content-projecting overlay panel. Toggled imperatively from a parent via a
 * template ref (`#ref`) exposing `toggle`/`show`/`hide`.
 *
 * The projected content is rendered inside a body-level CDK overlay via a
 * `TemplatePortal` over an `<ng-content>`-in-`<ng-template>` (same seam as
 * `ui-select`'s panel). No arrow.
 */
@Component({
    selector: 'ui-popover',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    template: `
        <ng-template #tpl>
            <!-- stopPropagation is intentionally NOT here: a nested ui-select's
                 own panel guards its clicks (§3b). This wrapper stays a plain
                 dismissible surface. -->
            <div class="ui-popover" [class]="panelClass()">
                <ng-content />
            </div>
        </ng-template>
    `
})
export class UiPopoverComponent implements OnDestroy {
    /** Extra class on both the overlay pane and the inner surface, so existing
     *  content-targeting CSS (e.g. `.notif-overlay-panel .ui-popover`) still hits. */
    public readonly panelClass = input<string>('');
    /** When false, outside clicks / Escape do NOT close (parity with a pinned popover). */
    public readonly dismissable = input(true);

    /** Fires on every close (outside click, Escape, `.hide()`, re-`.toggle()`). */
    public readonly onHide = output<void>();

    private readonly overlay = inject(Overlay);
    private readonly vcr = inject(ViewContainerRef);
    private readonly tpl = viewChild.required<TemplateRef<unknown>>('tpl');

    protected readonly isOpen = signal(false);
    private overlayRef: OverlayRef | null = null;
    /** Anchor element; excluded from outside-dismiss so its own toggle handler
     *  (not the CDK outside-click) governs closing — otherwise a click on the
     *  trigger would dismiss here and then immediately re-open via toggle(). */
    private originEl: HTMLElement | null = null;
    /** Extra element also excluded from outside-dismiss. Used by context-menu
     *  openers: the popover anchors to a synthetic point at the cursor, but the
     *  opening right-click's trailing `auxclick`/`pointerup` lands on the row —
     *  which would otherwise read as an outside click and close it on release. */
    private dismissExcludeEl: HTMLElement | null = null;

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

    /** Open anchored to an element (or the `currentTarget` of an event).
     *  `excludeEl` is an additional element to exclude from outside-dismiss
     *  (e.g. the row a context menu was opened from). */
    public show(target: HTMLElement | Event, excludeEl?: HTMLElement | null): void {
        if (this.isOpen()) {
            return;
        }
        const origin =
            target instanceof HTMLElement ? target : (target.currentTarget as HTMLElement | null);
        if (!origin) {
            return;
        }
        this.originEl = origin;
        this.dismissExcludeEl = excludeEl ?? null;

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
            const target = event.target as Node | null;
            const onOrigin =
                !!target &&
                ((!!this.originEl && this.originEl.contains(target)) ||
                    (!!this.dismissExcludeEl && this.dismissExcludeEl.contains(target)));
            if (this.dismissable() && !onOrigin) {
                this.hide();
            }
        });
        this.overlayRef.keydownEvents().subscribe(event => {
            if (event.key === 'Escape' && this.dismissable()) {
                this.hide();
            }
        });
        this.isOpen.set(true);
    }

    public hide(): void {
        if (!this.overlayRef) {
            return;
        }
        this.disposeOverlay();
        this.isOpen.set(false);
        this.onHide.emit();
    }

    /** Recompute the connected position — call after the projected content
     *  changes size (e.g. a master→detail view swap) so the panel re-anchors
     *  instead of staying placed for the old content. */
    public reposition(): void {
        this.overlayRef?.updatePosition();
    }

    private disposeOverlay(): void {
        this.overlayRef?.dispose();
        this.overlayRef = null;
        this.originEl = null;
        this.dismissExcludeEl = null;
    }
}

import { ConnectedPosition, Overlay, OverlayRef } from '@angular/cdk/overlay';
import { ComponentPortal } from '@angular/cdk/portal';
import {
    ComponentRef,
    Directive,
    ElementRef,
    OnDestroy,
    effect,
    inject,
    input
} from '@angular/core';
import {
    UiSaveState,
    UiSaveStatusChipComponent
} from '../components/save-status/save-status-chip.component';

/** How long the "Saved" chip lingers before fading out. */
const SAVED_DWELL_MS = 2400;

/** Floats at the field's top-right corner, above the border, out of flow. */
const POSITION: ConnectedPosition[] = [
    {
        originX: 'end',
        originY: 'top',
        overlayX: 'end',
        overlayY: 'bottom',
        offsetX: -8,
        offsetY: 10
    },
    {
        originX: 'end',
        originY: 'bottom',
        overlayX: 'end',
        overlayY: 'top',
        offsetX: -8,
        offsetY: -10
    }
];

/**
 * Quiet auto-save indicator (floating chip) for any form field — input, textarea,
 * or a `ui-select` host alike. Presentational only: the consumer owns the save and
 * drives `uiSaveStatus` through `idle → saving → saved` (or `error`). The chip lives
 * in a body-level CDK overlay anchored to the host, so it never shifts layout and
 * doesn't collide with in-field affordances (chevron, clear button).
 *
 * `saved` auto-fades after a short dwell; `error` stays until the next change.
 */
@Directive({
    selector: '[uiSaveStatus]',
    standalone: false
})
export class UiSaveStatusDirective implements OnDestroy {
    public readonly uiSaveStatus = input<UiSaveState>(UiSaveState.Idle);

    private readonly overlay = inject(Overlay);
    private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

    private overlayRef: OverlayRef | null = null;
    private componentRef: ComponentRef<UiSaveStatusChipComponent> | null = null;
    private dwellTimer: ReturnType<typeof setTimeout> | null = null;

    // Capture-phase so it catches scrolls in any (nested, non-cdkScrollable)
    // container — the chip closes rather than riding the scroll.
    private readonly onScroll = (): void => this.hide();

    public constructor() {
        effect(() => {
            const state = this.uiSaveStatus();
            this.clearDwell();
            if (state === UiSaveState.Idle) {
                this.hide();
                return;
            }
            this.show(state);
            if (state === UiSaveState.Saved) {
                this.dwellTimer = setTimeout(() => this.hide(), SAVED_DWELL_MS);
            }
        });
    }

    public ngOnDestroy(): void {
        this.clearDwell();
        this.hide();
    }

    private show(state: UiSaveState): void {
        if (!this.overlayRef) {
            this.overlayRef = this.overlay.create({
                positionStrategy: this.overlay
                    .position()
                    .flexibleConnectedTo(this.host)
                    .withPositions(POSITION),
                // Dismiss on scroll (like [uiTooltip]) rather than tracking the field —
                // a transient hint shouldn't ride the scroll. It reappears on the next
                // state change; detachments() tears our refs down so show() recreates it.
                scrollStrategy: this.overlay.scrollStrategies.close(),
                hasBackdrop: false
            });
            this.componentRef = this.overlayRef.attach(
                new ComponentPortal(UiSaveStatusChipComponent)
            );
            this.overlayRef.detachments().subscribe(() => this.hide());
            window.addEventListener('scroll', this.onScroll, { capture: true, passive: true });
        }
        this.componentRef!.setInput('state', state);
        this.componentRef!.changeDetectorRef.detectChanges();
    }

    private hide(): void {
        window.removeEventListener('scroll', this.onScroll, true);
        this.overlayRef?.dispose();
        this.overlayRef = null;
        this.componentRef = null;
    }

    private clearDwell(): void {
        if (this.dwellTimer) {
            clearTimeout(this.dwellTimer);
            this.dwellTimer = null;
        }
    }
}

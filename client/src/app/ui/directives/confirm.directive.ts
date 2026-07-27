import { Overlay, OverlayRef } from '@angular/cdk/overlay';
import { ComponentPortal } from '@angular/cdk/portal';
import {
    Directive,
    ElementRef,
    HostListener,
    OnDestroy,
    inject,
    input,
    output
} from '@angular/core';
import { Subscription } from 'rxjs';
import { UiConfirmPopupComponent } from '../components/confirm-popup/confirm-popup.component';

/**
 * Attach to a clickable element to show a confirmation popup anchored to it.
 *
 * The popup is positioned with CDK's flexible overlay strategy: it prefers
 * rendering below the host, and flips to the opposite side (top/right/left) when
 * the preferred side would overflow the viewport.
 *
 * Usage:
 * ```html
 * <ui-button uiConfirm (confirmed)="delete()" />
 * <ui-button uiConfirm [confirmText]="'X.CONFIRM' | translate" (confirmed)="delete()" />
 * ```
 */
@Directive({
    selector: '[uiConfirm]',
    standalone: false
})
export class UiConfirmDirective implements OnDestroy {
    /** Optional message override; defaults to `UI.CONFIRMATION.MESSAGE`. */
    public readonly confirmText = input<string>();
    public readonly acceptLabel = input<string>();
    public readonly rejectLabel = input<string>();

    /** Emitted when the user accepts. */
    public readonly confirmed = output<void>();

    private readonly overlay = inject(Overlay);
    private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

    private overlayRef: OverlayRef | null = null;
    private subscriptions = new Subscription();

    @HostListener('click')
    protected onClick(): void {
        if (this.overlayRef) {
            return;
        }
        this.open();
    }

    public ngOnDestroy(): void {
        this.close();
    }

    private open(): void {
        const positionStrategy = this.overlay
            .position()
            .flexibleConnectedTo(this.host)
            .withViewportMargin(8)
            .withFlexibleDimensions(false)
            .withGrowAfterOpen(false)
            .withPush(false)
            .withPositions([
                {
                    originX: 'center',
                    originY: 'bottom',
                    overlayX: 'center',
                    overlayY: 'top',
                    offsetY: 8
                },
                {
                    originX: 'center',
                    originY: 'top',
                    overlayX: 'center',
                    overlayY: 'bottom',
                    offsetY: -8
                },
                {
                    originX: 'end',
                    originY: 'center',
                    overlayX: 'start',
                    overlayY: 'center',
                    offsetX: 8
                },
                {
                    originX: 'start',
                    originY: 'center',
                    overlayX: 'end',
                    overlayY: 'center',
                    offsetX: -8
                }
            ]);

        this.overlayRef = this.overlay.create({
            positionStrategy,
            scrollStrategy: this.overlay.scrollStrategies.reposition(),
            hasBackdrop: false
        });

        this.subscriptions = new Subscription();
        const popup = this.overlayRef.attach(new ComponentPortal(UiConfirmPopupComponent));
        popup.setInput('text', this.confirmText());
        popup.setInput('acceptLabel', this.acceptLabel());
        popup.setInput('rejectLabel', this.rejectLabel());
        // Render synchronously with the inputs applied so the buttons paint fully
        // styled on first frame — otherwise they briefly show their initial
        // (label-less) state before the next change-detection tick.
        popup.changeDetectorRef.detectChanges();

        this.subscriptions.add(
            popup.instance.accepted.subscribe(() => {
                this.confirmed.emit();
                this.close();
            })
        );
        this.subscriptions.add(popup.instance.rejected.subscribe(() => this.close()));
        this.subscriptions.add(
            this.overlayRef.outsidePointerEvents().subscribe(() => this.close())
        );
        this.subscriptions.add(
            this.overlayRef.keydownEvents().subscribe(e => {
                if (e.key === 'Escape') {
                    this.close();
                }
            })
        );
    }

    private close(): void {
        this.subscriptions.unsubscribe();
        this.overlayRef?.dispose();
        this.overlayRef = null;
    }
}

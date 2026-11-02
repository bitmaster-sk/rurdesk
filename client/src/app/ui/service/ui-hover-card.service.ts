import { Overlay, OverlayRef } from '@angular/cdk/overlay';
import { ComponentPortal } from '@angular/cdk/portal';
import { ComponentRef, Injectable, OnDestroy, Type, inject } from '@angular/core';

@Injectable({ providedIn: 'root' })
export class UiHoverCard implements OnDestroy {
    public static readonly openDelay = 300;
    public static readonly closeDelay = 150;

    private readonly overlay = inject(Overlay);
    private overlayRef: OverlayRef | null = null;
    private anchor: HTMLElement | null = null;
    private openTimer: ReturnType<typeof setTimeout> | null = null;
    private closeTimer: ReturnType<typeof setTimeout> | null = null;

    public ngOnDestroy(): void {
        this.close();
    }

    public show<T>(
        anchor: HTMLElement,
        component: Type<T>,
        configure: (ref: ComponentRef<T>) => void
    ): void {
        this.cancelClose();
        if (this.anchor === anchor) {
            return;
        }
        this.cancelOpen();
        this.openTimer = setTimeout(() => {
            this.openTimer = null;
            this.open(anchor, component, configure);
        }, UiHoverCard.openDelay);
    }

    public hide(anchor: HTMLElement): void {
        this.cancelOpen();
        if (this.anchor !== anchor) {
            return;
        }
        this.scheduleClose();
    }

    public closeWithin(root: HTMLElement): void {
        if (this.anchor && root.contains(this.anchor)) {
            this.close();
        }
    }

    public close(): void {
        this.cancelOpen();
        this.cancelClose();
        this.overlayRef?.dispose();
        this.overlayRef = null;
        this.anchor = null;
    }

    private open<T>(
        anchor: HTMLElement,
        component: Type<T>,
        configure: (ref: ComponentRef<T>) => void
    ): void {
        this.close();
        if (!anchor.isConnected) {
            return;
        }
        const positionStrategy = this.overlay
            .position()
            .flexibleConnectedTo(anchor)
            .withViewportMargin(8)
            .withFlexibleDimensions(false)
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
                }
            ]);
        const overlayRef = this.overlay.create({
            positionStrategy,
            scrollStrategy: this.overlay.scrollStrategies.close(),
            panelClass: 'ui-hover-card-pane'
        });
        const ref = overlayRef.attach(new ComponentPortal(component));
        configure(ref);
        ref.changeDetectorRef.detectChanges();
        overlayRef.overlayElement.addEventListener('pointerenter', () => this.cancelClose());
        overlayRef.overlayElement.addEventListener('pointerleave', () => this.scheduleClose());
        overlayRef.keydownEvents().subscribe(event => {
            if (event.key === 'Escape') {
                this.close();
            }
        });
        this.overlayRef = overlayRef;
        this.anchor = anchor;
    }

    private scheduleClose(): void {
        this.cancelClose();
        this.closeTimer = setTimeout(() => {
            this.closeTimer = null;
            this.close();
        }, UiHoverCard.closeDelay);
    }

    private cancelOpen(): void {
        if (this.openTimer !== null) {
            clearTimeout(this.openTimer);
            this.openTimer = null;
        }
    }

    private cancelClose(): void {
        if (this.closeTimer !== null) {
            clearTimeout(this.closeTimer);
            this.closeTimer = null;
        }
    }
}

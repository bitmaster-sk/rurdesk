import { ConnectedPosition, Overlay, OverlayRef } from '@angular/cdk/overlay';
import { ComponentPortal } from '@angular/cdk/portal';
import {
    ComponentRef,
    Directive,
    ElementRef,
    HostListener,
    OnDestroy,
    effect,
    inject,
    input
} from '@angular/core';
import { UiTooltipComponent } from '../components/tooltip/tooltip.component';

export type UiTooltipPosition = 'top' | 'bottom' | 'left' | 'right';

const SHOW_DELAY_MS = 150;

/** Connected-position pairs per side, with the opposite side as a flip fallback. */
const POSITIONS: Record<UiTooltipPosition, ConnectedPosition[]> = {
    top: [
        { originX: 'center', originY: 'top', overlayX: 'center', overlayY: 'bottom', offsetY: -8 },
        { originX: 'center', originY: 'bottom', overlayX: 'center', overlayY: 'top', offsetY: 8 }
    ],
    bottom: [
        { originX: 'center', originY: 'bottom', overlayX: 'center', overlayY: 'top', offsetY: 8 },
        { originX: 'center', originY: 'top', overlayX: 'center', overlayY: 'bottom', offsetY: -8 }
    ],
    left: [
        { originX: 'start', originY: 'center', overlayX: 'end', overlayY: 'center', offsetX: -8 },
        { originX: 'end', originY: 'center', overlayX: 'start', overlayY: 'center', offsetX: 8 }
    ],
    right: [
        { originX: 'end', originY: 'center', overlayX: 'start', overlayY: 'center', offsetX: 8 },
        { originX: 'start', originY: 'center', overlayX: 'end', overlayY: 'center', offsetX: -8 }
    ]
};

/**
 * Hover/focus tooltip: a small dark bubble (`UiTooltipComponent`) anchored to
 * the host via a CDK overlay. No-op on empty text.
 */
@Directive({
    selector: '[uiTooltip]',
    standalone: false
})
export class UiTooltipDirective implements OnDestroy {
    public readonly uiTooltip = input<string>('');
    public readonly uiTooltipPosition = input<UiTooltipPosition>('top');

    private readonly overlay = inject(Overlay);
    private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

    private overlayRef: OverlayRef | null = null;
    private componentRef: ComponentRef<UiTooltipComponent> | null = null;
    private showTimer: ReturnType<typeof setTimeout> | null = null;
    private readonly tooltipId = `ui-tooltip-${nextId()}`;

    public constructor() {
        // Reactive text: if the label changes while the tooltip is shown, push it.
        effect(() => {
            const text = this.uiTooltip();
            if (this.componentRef) {
                this.componentRef.setInput('text', text);
                // The bubble lives in a detached overlay view — push CD explicitly,
                // deferred so it never nests inside an ongoing host change-detection.
                const ref = this.componentRef;
                queueMicrotask(() => ref.changeDetectorRef.detectChanges());
            }
            if (this.overlayRef && !text) {
                this.hide();
            }
        });
    }

    @HostListener('mouseenter')
    @HostListener('focus')
    protected onEnter(): void {
        this.scheduleShow();
    }

    @HostListener('mouseleave')
    @HostListener('blur')
    protected onLeave(): void {
        this.hide();
    }

    @HostListener('keydown.escape')
    protected onEscape(): void {
        this.hide();
    }

    public ngOnDestroy(): void {
        this.hide();
    }

    private scheduleShow(): void {
        if (this.overlayRef || this.showTimer || !this.uiTooltip()) {
            return;
        }
        this.showTimer = setTimeout(() => {
            this.showTimer = null;
            this.show();
        }, SHOW_DELAY_MS);
    }

    private show(): void {
        if (this.overlayRef || !this.uiTooltip()) {
            return;
        }
        const positionStrategy = this.overlay
            .position()
            .flexibleConnectedTo(this.host)
            .withPositions(POSITIONS[this.uiTooltipPosition()]);

        this.overlayRef = this.overlay.create({
            positionStrategy,
            scrollStrategy: this.overlay.scrollStrategies.close(),
            hasBackdrop: false
        });
        this.componentRef = this.overlayRef.attach(new ComponentPortal(UiTooltipComponent));
        this.componentRef.setInput('text', this.uiTooltip());
        this.componentRef.setInput('tooltipId', this.tooltipId);
        this.componentRef.changeDetectorRef.detectChanges();
        this.host.nativeElement.setAttribute('aria-describedby', this.tooltipId);
    }

    private hide(): void {
        if (this.showTimer) {
            clearTimeout(this.showTimer);
            this.showTimer = null;
        }
        if (!this.overlayRef) {
            return;
        }
        this.overlayRef.dispose();
        this.overlayRef = null;
        this.componentRef = null;
        this.host.nativeElement.removeAttribute('aria-describedby');
    }
}

let idCounter = 0;
function nextId(): number {
    return ++idCounter;
}

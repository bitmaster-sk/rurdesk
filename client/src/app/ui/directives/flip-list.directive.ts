import { AfterViewChecked, Directive, ElementRef, inject } from '@angular/core';
import { prefersReducedMotion, UI_SETTLE_DURATION_MS, UI_SETTLE_EASING } from '../util/motion';

const MIN_DELTA_PX = 2;

/**
 * FLIP list animation: when children marked with `data-flip-id` change vertical
 * position (sort change, reorder, refresh), they glide from their old position
 * to the new one instead of teleporting.
 *
 * Runs in ngAfterViewChecked, but only touches layout when the id ORDER of the
 * children changed — idle change-detection passes cost one attribute walk, no
 * reflow. Positions are tracked via `offsetTop` (scroll-independent) and are
 * re-baselined on every order change; elements whose id wasn't present before
 * (new rows) appear in place without animation. Honours `prefers-reduced-motion`.
 */
@Directive({
    selector: '[uiFlipList]',
    standalone: false
})
export class UiFlipListDirective implements AfterViewChecked {
    private readonly el = inject<ElementRef<HTMLElement>>(ElementRef);

    private positions = new Map<string, number>();
    private orderKey = '';

    public ngAfterViewChecked(): void {
        const items = this.el.nativeElement.querySelectorAll<HTMLElement>('[data-flip-id]');

        // Cheap guard: attribute reads only. Layout is touched solely when the
        // order actually changed since the last baseline.
        let nextOrderKey = '';
        items.forEach(item => (nextOrderKey += item.dataset['flipId'] + '|'));
        if (nextOrderKey === this.orderKey) return;
        this.orderKey = nextOrderKey;

        const previousPositions = this.positions;
        const nextPositions = new Map<string, number>();
        const isReduced = prefersReducedMotion();

        items.forEach(item => {
            const id = item.dataset['flipId'];
            if (!id) return;
            const top = item.offsetTop;
            nextPositions.set(id, top);

            const previous = previousPositions.get(id);
            if (!isReduced && previous !== undefined && Math.abs(previous - top) > MIN_DELTA_PX) {
                item.animate(
                    [
                        { transform: `translateY(${previous - top}px)` },
                        { transform: 'translateY(0)' }
                    ],
                    { duration: UI_SETTLE_DURATION_MS, easing: UI_SETTLE_EASING }
                );
            }
        });

        this.positions = nextPositions;
    }
}

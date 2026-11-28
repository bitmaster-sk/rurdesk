import { ConnectedPosition } from '@angular/cdk/overlay';

/** Where a menu or popover opens: under its trigger, or beside it (side menu rows). */
export type UiOverlayPlacement = 'below' | 'right';

const BELOW: ConnectedPosition[] = [
    { originX: 'start', originY: 'bottom', overlayX: 'start', overlayY: 'top', offsetY: 6 },
    { originX: 'start', originY: 'top', overlayX: 'start', overlayY: 'bottom', offsetY: -6 },
    { originX: 'end', originY: 'bottom', overlayX: 'end', overlayY: 'top', offsetY: 6 },
    { originX: 'end', originY: 'top', overlayX: 'end', overlayY: 'bottom', offsetY: -6 }
];

// Bottom edges aligned so a trigger low on the screen opens upward; top edges as the fallback.
const RIGHT: ConnectedPosition[] = [
    { originX: 'end', originY: 'bottom', overlayX: 'start', overlayY: 'bottom', offsetX: 8 },
    { originX: 'end', originY: 'top', overlayX: 'start', overlayY: 'top', offsetX: 8 }
];

export abstract class UiOverlayPosition {
    public static of(placement: UiOverlayPlacement): ConnectedPosition[] {
        return placement === 'right' ? RIGHT : BELOW;
    }
}

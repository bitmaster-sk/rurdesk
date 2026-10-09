import { describe, expect, it } from 'vitest';
import { UiLightboxZoom } from './lightbox-zoom';

describe('UiLightboxZoom.zoomAt', () => {
    it('zooms around the center without moving the image', () => {
        expect(UiLightboxZoom.zoomAt(UiLightboxZoom.initial, 2, 0, 0)).toEqual({
            scale: 2,
            x: 0,
            y: 0
        });
    });

    it('keeps the point under the cursor in place', () => {
        const zoomed = UiLightboxZoom.zoomAt(UiLightboxZoom.initial, 2, 100, 50);

        expect(zoomed).toEqual({ scale: 2, x: -100, y: -50 });
        expect(zoomed.x + zoomed.scale * 100).toBe(100);
        expect(zoomed.y + zoomed.scale * 50).toBe(50);
    });

    it('stops at the maximum scale', () => {
        expect(UiLightboxZoom.zoomAt({ scale: 6, x: 0, y: 0 }, 4, 0, 0).scale).toBe(
            UiLightboxZoom.maxScale
        );
    });

    it('snaps back to the fitted image when zoomed out fully', () => {
        expect(UiLightboxZoom.zoomAt({ scale: 1.5, x: 40, y: -20 }, 0.1, 10, 10)).toEqual(
            UiLightboxZoom.initial
        );
    });
});

describe('UiLightboxZoom.panBy', () => {
    it('moves a zoomed image', () => {
        expect(UiLightboxZoom.panBy({ scale: 2, x: 5, y: 5 }, 10, -20)).toEqual({
            scale: 2,
            x: 15,
            y: -15
        });
    });

    it('does not move a fitted image', () => {
        expect(UiLightboxZoom.panBy(UiLightboxZoom.initial, 10, 10)).toEqual(
            UiLightboxZoom.initial
        );
    });
});

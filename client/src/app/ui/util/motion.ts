// Shared motion vocabulary for programmatic (WAAPI) animations.
// SCSS keeps its own copies of these values — CSS can't be read from
// Element.animate() — so treat this file and the scss literals as one
// "settle" motion language and change them together.

/** The product's standard settle curve (ease-out with a soft landing). */
export const UI_SETTLE_EASING = 'cubic-bezier(0.22, 1, 0.36, 1)';

/** Standard duration for object movement (slides, flights, reorders) —
 *  matched to the gantt cascade slide, the pacing reference. */
export const UI_SETTLE_DURATION_MS = 420;

const reducedMotionQuery =
    typeof window !== 'undefined' ? window.matchMedia('(prefers-reduced-motion: reduce)') : null;

/** Live check of the user's reduced-motion preference (cached MediaQueryList). */
export function prefersReducedMotion(): boolean {
    return reducedMotionQuery?.matches ?? false;
}

/**
 * The product's "this just changed" signal: a one-shot primary ring that
 * expands and fades around the element. Used for inline updates in every view
 * (table row, kanban tile, gantt bar, calendar event) so a change reads the
 * same everywhere. `inset` suits full-bleed elements like table rows.
 */
export function pulseElement(el: HTMLElement, options: { inset?: boolean } = {}): void {
    if (prefersReducedMotion()) return;
    // A hidden tab has no frames to animate — skipping beats a stale pulse
    // replaying whenever the user comes back.
    if (document.hidden) return;
    const keyframes = options.inset
        ? [
              {
                  boxShadow:
                      'inset 0 0 0 3px color-mix(in srgb, var(--ui-color-primary) 55%, transparent)'
              },
              {
                  boxShadow:
                      'inset 0 0 0 3px color-mix(in srgb, var(--ui-color-primary) 0%, transparent)'
              }
          ]
        : [
              { boxShadow: '0 0 0 0 color-mix(in srgb, var(--ui-color-primary) 55%, transparent)' },
              { boxShadow: '0 0 0 7px color-mix(in srgb, var(--ui-color-primary) 0%, transparent)' }
          ];
    el.animate(keyframes, { duration: 700, easing: 'ease-out' });
}

/**
 * Normalises an arbitrary colour string to the strict `#rrggbb` form that the
 * native `<input type="color">` element requires. The browser silently coerces
 * anything else to `#000000` (and, crucially, `rrggbb` without the `#` too),
 * so we normalise up-front to keep the swatch showing the real colour.
 *
 * Accepts: `#rgb`, `rgb`, `#rrggbb`, `rrggbb` (any case, surrounding space).
 * Anything else (named colours, rgb()/hsl(), garbage) → `fallback`.
 */
export function normalizeHexColor(value: string | null | undefined, fallback = '#000000'): string {
    if (value == null) {
        return fallback;
    }

    const hex = value.trim().replace(/^#/, '').toLowerCase();

    if (/^[0-9a-f]{3}$/.test(hex)) {
        // Expand shorthand: `abc` → `aabbcc`.
        return `#${hex[0]}${hex[0]}${hex[1]}${hex[1]}${hex[2]}${hex[2]}`;
    }

    if (/^[0-9a-f]{6}$/.test(hex)) {
        return `#${hex}`;
    }

    return fallback;
}

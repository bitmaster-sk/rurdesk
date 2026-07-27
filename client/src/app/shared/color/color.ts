export abstract class Color {
    /**
     * A random, pleasant avatar background colour. Fixed saturation/lightness keep
     * it readable against white initials; only the hue varies. Returns `#rrggbb`.
     */
    public static randomAvatarBg(): string {
        const hue = Math.floor(Math.random() * 360);
        return Color.hslToHex(hue, 65, 55);
    }

    private static hslToHex(h: number, s: number, l: number): string {
        const sN = s / 100;
        const lN = l / 100;
        const k = (n: number): number => (n + h / 30) % 12;
        const a = sN * Math.min(lN, 1 - lN);
        const f = (n: number): number => {
            const color = lN - a * Math.max(-1, Math.min(k(n) - 3, Math.min(9 - k(n), 1)));
            return Math.round(255 * color);
        };
        const toHex = (v: number): string => v.toString(16).padStart(2, '0');
        return `#${toHex(f(0))}${toHex(f(8))}${toHex(f(4))}`;
    }

    public static getContrastColor(hex: string): string {
        const color = hex.replace(/^#/, '');
        const r = parseInt(color.substring(0, 2), 16);
        const g = parseInt(color.substring(2, 4), 16);
        const b = parseInt(color.substring(4, 6), 16);
        if (isNaN(r) || isNaN(g) || isNaN(b)) return '#1f2937';
        const luminance = 0.299 * r + 0.587 * g + 0.114 * b;
        return luminance > 128 ? '#1f2937' : '#ffffff';
    }
}

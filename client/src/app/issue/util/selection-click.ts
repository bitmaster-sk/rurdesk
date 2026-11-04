export type SelectionClickMode = 'toggle' | 'range' | null;

export abstract class SelectionClick {
    private static readonly interactive = 'a, button, input, select, textarea, [role="button"]';

    public static modeOf(event: MouseEvent): SelectionClickMode {
        const target = event.target as Element | null;
        if (target?.closest?.(SelectionClick.interactive)) return null;
        if (event.metaKey || event.ctrlKey) return 'toggle';
        if (event.shiftKey) return 'range';
        return null;
    }

    public static clearTextSelection(): void {
        window.getSelection()?.removeAllRanges();
    }
}

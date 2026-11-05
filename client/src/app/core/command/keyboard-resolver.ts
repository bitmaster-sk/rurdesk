import { HotkeyAction } from './command.model';

export abstract class KeyboardResolver {
    private static isEditableTarget(el: EventTarget | null): boolean {
        if (!el) return false;
        const node = el as HTMLElement;
        const tag = node.tagName;
        return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || node.isContentEditable;
    }

    /** An open CDK overlay (dropdown, menu, dialog) owns the keyboard */
    private static isInOverlay(el: EventTarget | null): boolean {
        const node = el as HTMLElement | null;
        return !!node?.closest?.('.cdk-overlay-container');
    }

    public static resolveHotkey(
        event: KeyboardEvent,
        ctx: { paletteOpen: boolean; helpOpen: boolean }
    ): HotkeyAction {
        const mod = event.metaKey || event.ctrlKey;
        if (mod && event.key.toLowerCase() === 'k') {
            return ctx.paletteOpen ? { type: 'close' } : { type: 'open', mode: 'all' };
        }
        if (ctx.helpOpen) return { type: 'none' }; // help sheet owns the screen
        if (ctx.paletteOpen) return { type: 'none' };
        if (
            KeyboardResolver.isEditableTarget(event.target) ||
            KeyboardResolver.isInOverlay(event.target) ||
            event.isComposing
        ) {
            return { type: 'none' };
        }
        const selection = KeyboardResolver.resolveSelection(event);
        if (selection) return selection;
        switch (event.key) {
            case '/':
                return { type: 'open', mode: 'navigation' };
            case '?':
                return { type: 'help' };
            case 'j':
            case 'ArrowDown':
                return { type: 'list-move', delta: 1 };
            case 'k':
            case 'ArrowUp':
                return { type: 'list-move', delta: -1 };
            case 'h':
            case 'ArrowLeft':
                return KeyboardResolver.hasModifier(event)
                    ? { type: 'none' }
                    : { type: 'list-move-across', delta: -1 };
            case 'l':
            case 'ArrowRight':
                return KeyboardResolver.hasModifier(event)
                    ? { type: 'none' }
                    : { type: 'list-move-across', delta: 1 };
            default:
                return { type: 'none' };
        }
    }

    private static hasModifier(event: KeyboardEvent): boolean {
        return event.metaKey || event.ctrlKey || event.altKey;
    }

    private static resolveSelection(event: KeyboardEvent): HotkeyAction | null {
        if (KeyboardResolver.hasModifier(event)) return null;
        if (event.shiftKey && (event.key === 'ArrowDown' || event.key === 'J')) {
            return { type: 'list-extend', delta: 1 };
        }
        if (event.shiftKey && (event.key === 'ArrowUp' || event.key === 'K')) {
            return { type: 'list-extend', delta: -1 };
        }
        switch (event.key) {
            case 'x':
                return { type: 'list-toggle' };
            case 'X':
                return { type: 'list-toggle-all' };
            case 'e':
                return { type: 'list-edit' };
            case 'Escape':
                return { type: 'list-clear' };
            default:
                return null;
        }
    }
}

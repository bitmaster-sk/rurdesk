import { HotkeyAction } from './command.model';

export function isEditableTarget(el: EventTarget | null): boolean {
    if (!el) return false;
    const node = el as HTMLElement;
    const tag = node.tagName;
    return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || node.isContentEditable;
}

/** An open CDK overlay (dropdown, menu, dialog) owns the keyboard */
export function isInOverlay(el: EventTarget | null): boolean {
    const node = el as HTMLElement | null;
    return !!node?.closest?.('.cdk-overlay-container');
}

export function resolveHotkey(
    event: KeyboardEvent,
    ctx: { paletteOpen: boolean; helpOpen: boolean }
): HotkeyAction {
    const mod = event.metaKey || event.ctrlKey;
    if (mod && event.key.toLowerCase() === 'k') {
        return ctx.paletteOpen ? { type: 'close' } : { type: 'open', mode: 'all' };
    }
    if (ctx.helpOpen) return { type: 'none' }; // help sheet owns the screen
    if (ctx.paletteOpen) return { type: 'none' };
    if (isEditableTarget(event.target) || isInOverlay(event.target) || event.isComposing) {
        return { type: 'none' };
    }
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
        default:
            return { type: 'none' };
    }
}

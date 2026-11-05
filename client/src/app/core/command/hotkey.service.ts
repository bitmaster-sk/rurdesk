import { Injectable, NgZone, inject } from '@angular/core';
import { KeyboardResolver } from './keyboard-resolver';
import { HotkeyAction } from './command.model';
import { CommandPaletteService } from './command-palette.service';

export interface ListNavigationHandler {
    move(delta: 1 | -1): void;
    moveAcross?(delta: 1 | -1): void;
}

export interface ListSelectionHandler {
    extend?(delta: 1 | -1): void;
    toggle?(): void;
    toggleAll?(): void;
    edit(): boolean;
    clear(): boolean;
}

const BLOCKING_OVERLAY =
    '.cdk-overlay-container [aria-modal="true"], .cdk-overlay-container .ui-popover, .cdk-overlay-container .ui-menu';

@Injectable({ providedIn: 'root' })
export class HotkeyService {
    private readonly palette = inject(CommandPaletteService);
    private readonly zone = inject(NgZone);
    private started = false;
    private listHandler: ListNavigationHandler | null = null;
    private selectionHandler: ListSelectionHandler | null = null;

    public registerListHandler(handler: ListNavigationHandler | null): void {
        this.listHandler = handler;
    }

    public registerSelectionHandler(handler: ListSelectionHandler | null): void {
        this.selectionHandler = handler;
    }

    public start(): void {
        if (this.started) return;
        this.started = true;
        // Register OUTSIDE Angular so plain typing (which resolves to 'none') never re-enters the
        // zone / triggers global change detection. Only a real action pays the zone.run cost.
        this.zone.runOutsideAngular(() =>
            document.addEventListener('keydown', e => this.onKeydown(e))
        );
    }

    private onKeydown(event: KeyboardEvent): void {
        const action = KeyboardResolver.resolveHotkey(event, {
            paletteOpen: this.palette.isOpen(),
            helpOpen: this.palette.isHelpOpen()
        });
        if (action.type === 'none') return; // common case: no zone entry, no CD
        const isListAction =
            action.type === 'list-move' ||
            action.type === 'list-move-across' ||
            HotkeyService.isSelectionAction(action);
        if (HotkeyService.isSelectionAction(action) && !this.selectionHandler) return;
        if (isListAction && HotkeyService.isOverlayOpen()) return;
        this.zone.run(() => {
            switch (action.type) {
                case 'open':
                    event.preventDefault();
                    this.palette.open(action.mode === 'navigation' ? '/' : '');
                    break;
                case 'close':
                    event.preventDefault();
                    this.palette.close();
                    break;
                case 'help':
                    event.preventDefault();
                    this.palette.openHelp();
                    break;
                case 'list-move':
                    if (this.listHandler) {
                        event.preventDefault();
                        this.listHandler.move(action.delta);
                    }
                    break;
                case 'list-move-across':
                    if (this.listHandler?.moveAcross) {
                        event.preventDefault();
                        this.listHandler.moveAcross(action.delta);
                    }
                    break;
                default:
                    this.runSelection(action, event);
            }
        });
    }

    private static isOverlayOpen(): boolean {
        return document.querySelector(BLOCKING_OVERLAY) !== null;
    }

    private static isSelectionAction(action: HotkeyAction): boolean {
        return (
            action.type === 'list-extend' ||
            action.type === 'list-toggle' ||
            action.type === 'list-toggle-all' ||
            action.type === 'list-edit' ||
            action.type === 'list-clear'
        );
    }

    private runSelection(action: HotkeyAction, event: KeyboardEvent): void {
        const handler = this.selectionHandler;
        if (!handler) return;
        let handled = false;
        switch (action.type) {
            case 'list-extend':
                handled = !!handler.extend;
                handler.extend?.(action.delta);
                break;
            case 'list-toggle':
                handled = !!handler.toggle;
                handler.toggle?.();
                break;
            case 'list-toggle-all':
                handled = !!handler.toggleAll;
                handler.toggleAll?.();
                break;
            case 'list-edit':
                handled = handler.edit();
                break;
            case 'list-clear':
                handled = handler.clear();
                break;
        }
        if (handled) event.preventDefault();
    }
}

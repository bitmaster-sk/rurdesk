import { describe, it, expect, beforeEach, vi } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { HotkeyService } from './hotkey.service';
import { CommandPaletteService } from './command-palette.service';

describe('HotkeyService', () => {
    let open: any, openHelp: any, close: any;
    beforeEach(() => {
        open = vi.fn();
        openHelp = vi.fn();
        close = vi.fn();
        TestBed.configureTestingModule({
            providers: [
                HotkeyService,
                {
                    provide: CommandPaletteService,
                    useValue: {
                        open,
                        openHelp,
                        close,
                        isOpen: () => false,
                        isHelpOpen: () => false
                    }
                }
            ]
        });
        TestBed.inject(HotkeyService).start();
    });
    it('opens palette on Cmd+K', () => {
        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', metaKey: true }));
        expect(open).toHaveBeenCalledWith('');
    });
    it('opens navigation on bare /', () => {
        document.dispatchEvent(new KeyboardEvent('keydown', { key: '/' }));
        expect(open).toHaveBeenCalledWith('/');
    });
    it('opens help on ?', () => {
        document.dispatchEvent(new KeyboardEvent('keydown', { key: '?' }));
        expect(openHelp).toHaveBeenCalled();
    });
    it('does not fire when typing in an input', () => {
        const i = document.createElement('input');
        document.body.appendChild(i);
        i.focus();
        i.dispatchEvent(new KeyboardEvent('keydown', { key: '/', bubbles: true }));
        expect(open).not.toHaveBeenCalled();
        i.remove();
    });
    it('routes selection keys to the registered selection handler', () => {
        const handler = {
            toggle: vi.fn(),
            edit: vi.fn(() => true),
            clear: vi.fn(() => false)
        };
        TestBed.inject(HotkeyService).registerSelectionHandler(handler);

        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'x' }));
        const edit = new KeyboardEvent('keydown', { key: 'e', cancelable: true });
        document.dispatchEvent(edit);
        const esc = new KeyboardEvent('keydown', { key: 'Escape', cancelable: true });
        document.dispatchEvent(esc);

        expect(handler.toggle).toHaveBeenCalledTimes(1);
        expect(handler.edit).toHaveBeenCalledTimes(1);
        expect(edit.defaultPrevented).toBe(true);
        expect(handler.clear).toHaveBeenCalledTimes(1);
        expect(esc.defaultPrevented).toBe(false);
        TestBed.inject(HotkeyService).registerSelectionHandler(null);
    });

    it('ignores list and selection keys while a dialog or popover is open', () => {
        const service = TestBed.inject(HotkeyService);
        const move = vi.fn();
        const handler = { toggle: vi.fn(), edit: vi.fn(() => true), clear: vi.fn(() => true) };
        service.registerListHandler({ move });
        service.registerSelectionHandler(handler);
        const container = document.createElement('div');
        container.className = 'cdk-overlay-container';
        container.innerHTML = '<div role="dialog" aria-modal="true"></div>';
        document.body.appendChild(container);

        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown' }));
        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'x' }));
        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));

        expect(move).not.toHaveBeenCalled();
        expect(handler.toggle).not.toHaveBeenCalled();
        expect(handler.clear).not.toHaveBeenCalled();

        container.remove();
        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'x' }));
        expect(handler.toggle).toHaveBeenCalled();
        service.registerListHandler(null);
        service.registerSelectionHandler(null);
    });

    it('routes left/right to moveAcross only when the list supports it', () => {
        const service = TestBed.inject(HotkeyService);
        const move = vi.fn();
        const moveAcross = vi.fn();
        service.registerListHandler({ move });
        const plain = new KeyboardEvent('keydown', { key: 'ArrowRight', cancelable: true });
        document.dispatchEvent(plain);
        expect(plain.defaultPrevented).toBe(false);

        service.registerListHandler({ move, moveAcross });
        const across = new KeyboardEvent('keydown', { key: 'ArrowLeft', cancelable: true });
        document.dispatchEvent(across);
        expect(moveAcross).toHaveBeenCalledWith(-1);
        expect(across.defaultPrevented).toBe(true);
        service.registerListHandler(null);
    });
});

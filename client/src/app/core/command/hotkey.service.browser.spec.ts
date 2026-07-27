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
});

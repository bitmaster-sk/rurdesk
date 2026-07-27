import { Injectable, NgZone, inject } from '@angular/core';
import { resolveHotkey } from './keyboard.util';
import { CommandPaletteService } from './command-palette.service';

@Injectable({ providedIn: 'root' })
export class HotkeyService {
    private readonly palette = inject(CommandPaletteService);
    private readonly zone = inject(NgZone);
    private started = false;
    private listHandler: ((delta: 1 | -1) => void) | null = null;

    public registerListHandler(handler: ((delta: 1 | -1) => void) | null): void {
        this.listHandler = handler;
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
        const action = resolveHotkey(event, {
            paletteOpen: this.palette.isOpen(),
            helpOpen: this.palette.isHelpOpen()
        });
        if (action.type === 'none') return; // common case: no zone entry, no CD
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
                        this.listHandler(action.delta);
                    }
                    break;
            }
        });
    }
}

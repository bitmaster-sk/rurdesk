import { describe, it, expect, beforeEach } from 'vitest';
import { ApplicationRef } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { UiModule } from '../../ui/ui.module';
import { CommandPaletteService } from './command-palette.service';
import { CommandRegistryService } from './command-registry.service';
import { Command } from './command.model';

const cmd = (id: string, title: string, run: () => void): Command => ({
    id,
    title,
    group: 'G',
    icon: 'command',
    modes: ['all', 'commands'],
    run
});

// The palette renders through a CDK overlay portal, whose host view Angular renders on the
// next application tick — call tick() after each open()/click to flush it into the DOM.
const tick = () => TestBed.inject(ApplicationRef).tick();

describe('CommandPaletteService', () => {
    let ran: string[];
    beforeEach(() => {
        ran = [];
        localStorage.clear();
        TestBed.configureTestingModule({
            imports: [UiModule, TranslateModule.forRoot()],
            providers: [CommandPaletteService, CommandRegistryService]
        });
        TestBed.inject(CommandRegistryService).register({
            getCommands: () => [cmd('a', 'Alpha', () => ran.push('a'))]
        });
    });

    it('opens and renders provider commands', () => {
        const svc = TestBed.inject(CommandPaletteService);
        svc.setContext({ idProject: 1, issue: null });
        svc.open();
        tick();
        expect(svc.isOpen()).toBe(true);
        expect(document.querySelector('[data-item="a"]')).not.toBeNull();
        svc.close();
    });
    it('runs and closes on execute', () => {
        const svc = TestBed.inject(CommandPaletteService);
        svc.setContext({ idProject: 1, issue: null });
        svc.open();
        tick();
        (document.querySelector('[data-item="a"]') as HTMLElement).click();
        expect(ran).toEqual(['a']);
        expect(svc.isOpen()).toBe(false);
    });
    it('opens navigation mode when prefilled with "/"', () => {
        const svc = TestBed.inject(CommandPaletteService);
        svc.setContext({ idProject: 1, issue: null });
        svc.open('/');
        tick();
        expect(document.querySelector('.palette__mode')?.textContent).toContain(
            'COMMAND.MODE.NAVIGATION'
        ); // translate echoes key when unloaded
        svc.close();
    });
    it('does not offer create in a mode the create command does not target', () => {
        // register a create provider scoped to all/issues only
        TestBed.inject(CommandRegistryService).register({
            getCommands: () => [],
            createFromQuery: (q, ctx) =>
                ctx.idProject == null
                    ? null
                    : {
                          id: 'issue.create',
                          title: `Create „${q}"`,
                          group: 'Create',
                          icon: 'plus',
                          modes: ['all', 'issues'],
                          run: () => {}
                      }
        });
        const svc = TestBed.inject(CommandPaletteService);
        svc.setContext({ idProject: 1, issue: null });
        svc.open('/zzz');
        tick(); // navigation mode, no hit
        expect(document.querySelector('[data-item="issue.create"]')).toBeNull();
        svc.close();
    });
});

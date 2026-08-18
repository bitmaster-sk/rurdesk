import { ComponentRef, Injectable, computed, inject, signal } from '@angular/core';
import { Overlay, OverlayRef } from '@angular/cdk/overlay';
import { ComponentPortal } from '@angular/cdk/portal';
import { Subscription } from 'rxjs';
import { UiCommandPaletteComponent } from '../../ui/components/command-palette/command-palette.component';
import { UiCommandHelpComponent } from '../../ui/components/command-help/command-help.component';
import { buildGroups } from './build-groups.util';
import { parseQuery } from './parse-query.util';
import { MODE_PREFIX, CommandContext, RankedCommand } from './command.model';
import { CommandRegistryService } from './command-registry.service';
import { RecentCommandsStore } from './recent-commands.store';

@Injectable({ providedIn: 'root' })
export class CommandPaletteService {
    private readonly overlay = inject(Overlay);
    private readonly registry = inject(CommandRegistryService);
    private readonly recents = inject(RecentCommandsStore);

    private overlayRef: OverlayRef | null = null;
    private ref: ComponentRef<UiCommandPaletteComponent> | null = null;
    private helpRef: OverlayRef | null = null;
    private subs = new Subscription();

    private readonly open$ = signal(false);
    private readonly helpOpen$ = signal(false);
    private readonly query$ = signal('');
    private context: CommandContext = { idProject: null, issue: null };

    /** Palette-only — drives the ⌘K toggle semantics. */
    public readonly isOpen = computed(() => this.open$());
    /** Help-only — fed into the hotkey gate so `?`/`/`/`j` are suppressed while help is up. */
    public readonly isHelpOpen = computed(() => this.helpOpen$());
    /** Any command overlay open — the Gantt gate defers to this. */
    public readonly isOverlayOpen = computed(() => this.open$() || this.helpOpen$());

    public setContext(ctx: CommandContext): void {
        this.context = ctx;
    }

    public open(prefill = ''): void {
        if (this.overlayRef) this.close();
        this.query$.set(prefill);
        this.overlayRef = this.overlay.create({
            hasBackdrop: true,
            backdropClass: 'command-palette-backdrop',
            positionStrategy: this.overlay.position().global().centerHorizontally().top('12vh'),
            scrollStrategy: this.overlay.scrollStrategies.block()
        });
        this.ref = this.overlayRef.attach(new ComponentPortal(UiCommandPaletteComponent));
        this.open$.set(true);

        const inst = this.ref.instance;
        this.subs = new Subscription();
        this.subs.add(
            inst.queryChange.subscribe(v => {
                this.query$.set(v);
                this.recompute();
            })
        );
        this.subs.add(inst.execute.subscribe(c => this.run(c, false)));
        this.subs.add(inst.executePersist.subscribe(c => this.run(c, true)));
        this.subs.add(inst.complete.subscribe(c => this.completeWith(c)));
        this.subs.add(inst.closed.subscribe(() => this.close()));
        this.subs.add(this.overlayRef.backdropClick().subscribe(() => this.close()));

        this.registry.prime(this.context).subscribe(() => this.recompute());
        this.recompute();
        queueMicrotask(() => this.ref?.instance.focusInput());
    }

    public close(): void {
        this.subs.unsubscribe();
        this.overlayRef?.dispose();
        this.overlayRef = null;
        this.ref = null;
        this.open$.set(false);
    }

    public openHelp(): void {
        if (this.helpRef) return;
        this.helpRef = this.overlay.create({
            hasBackdrop: true,
            backdropClass: 'command-palette-backdrop',
            positionStrategy: this.overlay
                .position()
                .global()
                .centerHorizontally()
                .centerVertically()
        });
        const help = this.helpRef.attach(new ComponentPortal(UiCommandHelpComponent));
        this.helpOpen$.set(true);
        const closeHelp = (): void => {
            this.helpRef?.dispose();
            this.helpRef = null;
            this.helpOpen$.set(false);
        };
        help.instance.closed.subscribe(closeHelp);
        this.helpRef.backdropClick().subscribe(closeHelp);
        // CDK routes document keydowns to the topmost overlay regardless of focus.
        this.helpRef.keydownEvents().subscribe(e => {
            if (e.key === 'Escape') closeHelp();
        });
    }

    private recompute(): void {
        if (!this.ref) return;
        const { mode, query } = parseQuery(this.query$());
        const groups = buildGroups(
            this.registry.collect(this.context),
            mode,
            query,
            this.recents.recentIds()
        );
        // Offer create only when a create command actually applies to the current mode
        // (create commands carry ['all','issues'] → never shown in / > @).
        const create =
            groups.length === 0 && query
                ? (this.registry
                      .createCommands(query, this.context)
                      .find(c => mode === 'all' || c.modes.includes(mode)) ?? null)
                : null;
        this.ref.setInput('mode', mode);
        this.ref.setInput('query', this.query$());
        this.ref.setInput('groups', groups);
        this.ref.setInput(
            'createItem',
            create
                ? {
                      ...create,
                      score: 0,
                      highlight: [{ text: create.title, hit: false }]
                  }
                : null
        );
    }

    private run(command: RankedCommand, persist: boolean): void {
        this.recents.push(command.id);
        if (!persist) this.close();
        command.run();
        if (persist) this.recompute();
    }

    private completeWith(command: RankedCommand): void {
        const { mode } = parseQuery(this.query$());
        const prefix = mode === 'all' ? '' : MODE_PREFIX[mode];
        // Use the command's own completion token when set (e.g. '#428'), else its title.
        // The token already carries its prefix, so only add the mode prefix when it doesn't.
        const token = command.completion ?? command.title;
        const value = token.startsWith(prefix) ? token : `${prefix}${token}`;
        this.query$.set(`${value} `);
        this.recompute();
        this.ref?.setInput('query', this.query$());
    }
}

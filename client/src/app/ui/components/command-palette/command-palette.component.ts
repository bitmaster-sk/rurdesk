import {
    ChangeDetectionStrategy,
    Component,
    ElementRef,
    computed,
    effect,
    inject,
    input,
    output,
    signal,
    viewChild
} from '@angular/core';
import { CommandGroup, CommandMode, RankedCommand } from '../../../core/command/command.model';

@Component({
    selector: 'ui-command-palette',
    templateUrl: './command-palette.component.html',
    styleUrls: ['./command-palette.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class UiCommandPaletteComponent {
    public readonly groups = input<CommandGroup[]>([]);
    public readonly mode = input<CommandMode>('all');
    public readonly query = input('');
    public readonly createItem = input<RankedCommand | null>(null);

    public readonly queryChange = output<string>();
    public readonly execute = output<RankedCommand>();
    public readonly executePersist = output<RankedCommand>();
    public readonly complete = output<RankedCommand>();
    public readonly closed = output<void>();

    private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
    private readonly inputRef = viewChild.required<ElementRef<HTMLInputElement>>('input');

    protected readonly flat = computed<RankedCommand[]>(() => {
        const items = this.groups().flatMap(g => g.items);
        const create = this.createItem();
        return items.length === 0 && create ? [create] : items;
    });
    protected readonly selected = signal(0);
    /** Translation KEYS (chrome is translated at render time via `| translate`, unlike command
     *  titles which are pre-translated by the builders to stay searchable). */
    protected readonly labelKeys: Record<CommandMode, string> = {
        all: 'COMMAND.MODE.ALL',
        commands: 'COMMAND.MODE.COMMANDS',
        people: 'COMMAND.MODE.PEOPLE',
        issues: 'COMMAND.MODE.ISSUES',
        navigation: 'COMMAND.MODE.NAVIGATION'
    };

    public constructor() {
        effect(() => {
            this.groups();
            this.createItem();
            this.selected.set(0);
        });
    }

    public focusInput(): void {
        this.inputRef().nativeElement.focus();
    }

    protected isSelected(item: RankedCommand): boolean {
        return this.flat()[this.selected()]?.id === item.id;
    }

    protected onInput(value: string): void {
        this.queryChange.emit(value);
    }

    protected onKeydown(event: KeyboardEvent): void {
        const items = this.flat();
        const current = items[this.selected()];
        switch (event.key) {
            case 'ArrowDown':
                event.preventDefault();
                this.selected.update(i => Math.min(i + 1, items.length - 1));
                this.scrollSelectedIntoView();
                break;
            case 'ArrowUp':
                event.preventDefault();
                this.selected.update(i => Math.max(i - 1, 0));
                this.scrollSelectedIntoView();
                break;
            case 'Enter':
                event.preventDefault();
                if (current)
                    (event.metaKey || event.ctrlKey ? this.executePersist : this.execute).emit(
                        current
                    );
                break;
            case 'Tab':
                event.preventDefault();
                if (current) this.complete.emit(current);
                break;
            case 'Escape':
                event.preventDefault();
                if (this.query()) this.queryChange.emit('');
                else this.closed.emit();
                break;
        }
    }

    protected onItemClick(item: RankedCommand): void {
        this.execute.emit(item);
    }

    /** Keep the selected row visible when moving past the scroll viewport edge. */
    private scrollSelectedIntoView(): void {
        queueMicrotask(() =>
            this.host.nativeElement
                .querySelector('.palette__item.sel')
                ?.scrollIntoView({ block: 'nearest' })
        );
    }
}

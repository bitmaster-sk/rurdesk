import { ChangeDetectionStrategy, Component, computed, output, signal } from '@angular/core';
import { WikiTableSize } from '../../entity/wiki-table-size.entity';

@Component({
    selector: 'app-wiki-table-picker',
    templateUrl: './wiki-table-picker.component.html',
    styleUrls: ['./wiki-table-picker.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class WikiTablePickerComponent {
    public readonly picked = output<WikiTableSize>();

    protected readonly maxRows = 8;
    protected readonly maxColumns = 8;
    protected readonly rows = Array.from({ length: this.maxRows }, (_, index) => index + 1);
    protected readonly columns = Array.from({ length: this.maxColumns }, (_, index) => index + 1);
    protected readonly size = signal<WikiTableSize>({ rows: 3, columns: 3 });
    protected readonly label = computed(() => `${this.size().rows} × ${this.size().columns}`);

    protected isActive(row: number, column: number): boolean {
        return row <= this.size().rows && column <= this.size().columns;
    }

    protected onHover(row: number, column: number): void {
        this.size.set({ rows: row, columns: column });
    }

    protected onPick(row: number, column: number): void {
        this.picked.emit({ rows: row, columns: column });
    }

    protected onKeydown(event: KeyboardEvent): void {
        const { rows, columns } = this.size();
        const moves: Record<string, WikiTableSize> = {
            ArrowUp: { rows: Math.max(1, rows - 1), columns },
            ArrowDown: { rows: Math.min(this.maxRows, rows + 1), columns },
            ArrowLeft: { rows, columns: Math.max(1, columns - 1) },
            ArrowRight: { rows, columns: Math.min(this.maxColumns, columns + 1) }
        };
        if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault();
            this.picked.emit(this.size());
            return;
        }
        const next = moves[event.key];
        if (next) {
            event.preventDefault();
            this.size.set(next);
        }
    }
}

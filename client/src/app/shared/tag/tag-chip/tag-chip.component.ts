import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';

@Component({
    selector: 'app-tag-chip',
    templateUrl: './tag-chip.component.html',
    styleUrls: ['./tag-chip.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class TagChipComponent {
    public readonly name = input.required<string>();
    public readonly color = input<string | null>(null);
    public readonly isRemovable = input(false);
    public readonly removeLabel = input<string>('');

    public readonly removed = output<void>();

    protected onRemove(event: MouseEvent): void {
        event.stopPropagation();
        this.removed.emit();
    }
}

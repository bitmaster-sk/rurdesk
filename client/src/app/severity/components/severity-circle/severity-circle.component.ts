import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

@Component({
    selector: 'app-severity-circle',
    templateUrl: './severity-circle.component.html',
    styleUrls: ['./severity-circle.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class SeverityCircleComponent {
    public readonly color = input<string | null | undefined>(undefined);
    public readonly size = input<number>(1);
    public readonly sizeUnit = input<string>('rem');

    protected readonly circleStyle = computed(() => ({
        width: `${this.size()}${this.sizeUnit()}`,
        height: `${this.size()}${this.sizeUnit()}`,
        backgroundColor: this.color() ?? 'var(--ui-surface-200)'
    }));
}

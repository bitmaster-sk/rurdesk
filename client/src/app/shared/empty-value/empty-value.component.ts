import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { EmptyValueAlign } from './constant/empty-value-align.enum';

@Component({
    selector: 'app-empty-value',
    templateUrl: './empty-value.component.html',
    styleUrls: ['./empty-value.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class EmptyValueComponent {
    public readonly align = input<EmptyValueAlign>(EmptyValueAlign.Text);

    protected readonly alignClass = computed(() => `empty-value--${this.align()}`);
}

import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

export interface WorkloadEntry {
    name: string;
    count: number;
    bgColor?: string;
    isUnassigned: boolean;
}

@Component({
    selector: 'app-workload-bar-list',
    templateUrl: './workload-bar-list.component.html',
    styleUrls: ['./workload-bar-list.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class WorkloadBarListComponent {
    public entries = input.required<WorkloadEntry[]>();

    private max = computed(() => Math.max(1, ...this.entries().map(entry => entry.count)));

    public rows = computed(() =>
        this.entries().map(entry => ({
            ...entry,
            percent: Math.round((entry.count / this.max()) * 100)
        }))
    );

    public isEmpty = computed(
        () => this.entries().length === 0 || this.entries().every(entry => entry.count === 0)
    );
}

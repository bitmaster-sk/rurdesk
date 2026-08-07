import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { ChartData, ChartOptions } from 'chart.js';
import { StatsChartEntry } from '../project-stats-chart/project-stats-chart.component';

type ValueFormat = 'number' | 'hours';

@Component({
    selector: 'app-stats-bar-chart',
    templateUrl: './stats-bar-chart.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class StatsBarChartComponent {
    public entries = input.required<StatsChartEntry[]>();
    public colors = input<string[]>();
    public valueFormat = input<ValueFormat>('number');

    public chartData = computed<ChartData<'bar'>>(() => {
        const data = this.entries();
        const customColors = this.colors();
        const values =
            this.valueFormat() === 'hours'
                ? data.map(entry => entry.value / 3600)
                : data.map(entry => entry.value);

        return {
            labels: data.map(entry => entry.name),
            datasets: [
                {
                    data: values,
                    ...(customColors?.length ? { backgroundColor: customColors } : {}),
                    borderWidth: 0,
                    borderRadius: 4,
                    barThickness: 18
                }
            ]
        };
    });

    public chartOptions = computed<ChartOptions<'bar'>>(() => {
        const format = this.valueFormat();
        return {
            indexAxis: 'y',
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                x: { beginAtZero: true, grid: { display: false } },
                y: { grid: { display: false } }
            },
            plugins: {
                legend: { display: false },
                tooltip: {
                    callbacks: {
                        label: ctx => {
                            const value = ctx.parsed.x ?? 0;
                            if (format === 'hours') {
                                const h = Math.floor(value);
                                const m = Math.round((value - h) * 60);
                                return ` ${h}h ${m}m`;
                            }
                            return ` ${value}`;
                        }
                    }
                }
            }
        };
    });

    public heightPx = computed(() => Math.max(120, this.entries().length * 36));

    public isEmpty = computed(() => {
        const data = this.entries();
        return data.length === 0 || data.every(entry => entry.value === 0);
    });
}

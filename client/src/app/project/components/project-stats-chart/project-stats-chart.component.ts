import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { ChartData, ChartOptions } from 'chart.js';

export interface StatsChartEntry {
    name: string;
    value: number;
}

@Component({
    selector: 'app-project-stats-chart',
    templateUrl: './project-stats-chart.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ProjectStatsChartComponent {
    public label = input.required<string>();
    public entries = input.required<StatsChartEntry[]>();
    public colors = input<string[]>();

    public chartData = computed<ChartData<'doughnut'>>(() => {
        const data = this.entries();
        const customColors = this.colors();

        return {
            labels: data.map(e => e.name),
            datasets: [
                {
                    data: data.map(e => e.value),
                    ...(customColors?.length ? { backgroundColor: customColors } : {}),
                    borderWidth: 0
                }
            ]
        };
    });

    public chartOptions: ChartOptions<'doughnut'> = {
        responsive: true,
        maintainAspectRatio: false,
        cutout: '60%',
        plugins: {
            legend: {
                display: true,
                position: 'right',
                labels: {
                    boxWidth: 10,
                    boxHeight: 10,
                    padding: 10,
                    font: { size: 12 },
                    generateLabels: chart => {
                        const data = chart.data;
                        return (data.labels ?? []).map((label, i) => {
                            const meta = chart.getDatasetMeta(0);
                            const value = (data.datasets[0].data[i] as number) ?? 0;
                            const style = meta.controller.getStyle(i, false) as {
                                backgroundColor: string;
                                borderColor: string;
                            };
                            // chart.js types labels as unknown[]; this chart always feeds
                            // them from StatsChartEntry.name.
                            const text = typeof label === 'string' ? label : '';
                            return {
                                text: `${text} (${value})`,
                                fillStyle: style.backgroundColor,
                                strokeStyle: style.borderColor,
                                hidden: !chart.getDataVisibility(i),
                                index: i
                            };
                        });
                    }
                }
            },
            tooltip: {
                callbacks: { label: ctx => `${ctx.label}: ${ctx.parsed}` }
            }
        }
    };

    public total = computed(() => this.entries().reduce((sum, e) => sum + e.value, 0));
}

import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import { ChartData, ChartOptions } from 'chart.js';
import { Track } from 'src/app/shared/tracker/model/track.model';
import { addDays, subDays } from 'date-fns';

interface IssueAggregate {
    idIssue: number;
    idIssuePublic: number;
    issueTitle: string;
    hoursPerDay: Map<string, number>;
}

@Component({
    selector: 'app-tracked-time-chart',
    templateUrl: './tracked-time-chart.component.html',
    styleUrls: ['./tracked-time-chart.component.scss'],
    providers: [DatePipe],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class TrackedTimeChartComponent {
    private readonly datePipe = inject(DatePipe);

    public tracks = input.required<Track[]>();
    public days = input<number>(7);
    public from = input<Date | null>(null);
    public yAxisLabel = input<string>('');

    private dayLabels = computed(() => {
        const labels: string[] = [];
        const startDate = this.from() ?? subDays(new Date(), this.days() - 1);
        let date = new Date(startDate);
        for (let i = 0; i < this.days(); i++) {
            labels.push(this.datePipe.transform(date, 'mediumDate')!);
            date = addDays(date, 1);
        }
        return labels;
    });

    private issueAggregates = computed(() => {
        const labels = this.dayLabels();
        const issueMap = new Map<number, IssueAggregate>();

        for (const t of this.tracks()) {
            if (!t.startAt || !t.tracked) {
                continue;
            }
            const dayKey = this.datePipe.transform(t.startAt, 'mediumDate')!;
            if (!labels.includes(dayKey)) {
                continue;
            }

            let agg = issueMap.get(t.idIssue);
            if (!agg) {
                agg = {
                    idIssue: t.idIssue,
                    idIssuePublic: t.idIssuePublic,
                    issueTitle: t.issueTitle,
                    hoursPerDay: new Map()
                };
                issueMap.set(t.idIssue, agg);
            }
            const current = agg.hoursPerDay.get(dayKey) ?? 0;
            agg.hoursPerDay.set(dayKey, current + t.tracked / 3600);
        }

        // Sort by idIssue for stable dataset ordering within a view.
        // Chart.js Colors plugin assigns colors by dataset index, so a stable
        // order guarantees same issue = same color for the entire view lifetime.
        return Array.from(issueMap.values()).sort((a, b) => a.idIssue - b.idIssue);
    });

    public chartData = computed<ChartData<'bar'>>(() => {
        const labels = this.dayLabels();
        const aggregates = this.issueAggregates();

        const datasets = aggregates.map(agg => ({
            label: `#${agg.idIssuePublic} ${agg.issueTitle}`,
            data: labels.map(day => agg.hoursPerDay.get(day) ?? 0),
            borderWidth: 0,
            borderRadius: 2
        }));

        return { labels, datasets };
    });

    public chartOptions = computed<ChartOptions<'bar'>>(() => ({
        responsive: true,
        maintainAspectRatio: false,
        scales: {
            x: { stacked: true, grid: { display: false } },
            y: {
                stacked: true,
                beginAtZero: true,
                suggestedMax: 8,
                title: { display: !!this.yAxisLabel(), text: this.yAxisLabel() }
            }
        },
        plugins: {
            legend: { display: false },
            tooltip: {
                callbacks: {
                    title: items => items[0]?.label ?? '',
                    label: ctx => {
                        const hours = ctx.parsed.y ?? 0;
                        const h = Math.floor(hours);
                        const m = Math.round((hours - h) * 60);
                        return ` ${ctx.dataset.label}: ${h}h ${m}m`;
                    }
                }
            }
        }
    }));

    public heightPx = computed(() => {
        const issueCount = this.issueAggregates().length;
        const minHeight = 195;
        const perIssue = 26;
        return Math.max(minHeight, issueCount * perIssue);
    });

    public isEmpty = computed(() => this.issueAggregates().length === 0);
}

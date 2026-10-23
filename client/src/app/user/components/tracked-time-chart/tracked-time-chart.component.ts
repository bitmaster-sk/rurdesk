import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, input, signal } from '@angular/core';
import { ActiveElement, ChartData, ChartOptions } from 'chart.js';
import { DurationConverter } from 'src/app/shared/duration/duration.converter';
import { DurationFormatter } from 'src/app/shared/duration/duration.formatter';
import { Track } from 'src/app/shared/tracker/model/track.model';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';
import { TrackedTimeChartDataset } from 'src/app/user/entity/tracked-time-chart-dataset.entity';
import { addDays, subDays } from 'date-fns';
import { TrackedTimeChartPlugins } from './tracked-time-chart-plugins';
import { TrackedTimeChartSelection } from './tracked-time-chart-selection';
import { TrackedTimeChartText } from './tracked-time-chart-text';

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

    private readonly trackerService = inject(TrackerService);

    public tracks = input.required<Track[]>();
    public days = input<number>(7);
    public from = input<Date | null>(null);
    public yAxisLabel = input<string>('');

    public readonly chartPlugins = [
        TrackedTimeChartPlugins.segmentLabels,
        TrackedTimeChartPlugins.dayTotals
    ];

    protected readonly selection = signal<{ idIssue: number; dayKey: string } | null>(null);

    protected readonly editingId = signal<number | null>(null);

    protected readonly selectedTracks = computed(() => {
        const selection = this.selection();
        if (selection === null) {
            return [];
        }
        return TrackedTimeChartSelection.forSegment(
            this.tracks(),
            selection.idIssue,
            selection.dayKey,
            this.toDayKey
        );
    });

    protected readonly isDialogOpen = computed(
        () => this.selection() !== null && this.selectedTracks().length > 0
    );

    protected readonly isSingle = computed(() => this.selectedTracks().length === 1);

    protected readonly selectedTotal = computed(() =>
        this.selectedTracks().reduce((sum, t) => sum + (t.tracked ?? 0), 0)
    );

    private readonly toDayKey = (d: Date): string => this.datePipe.transform(d, 'mediumDate')!;

    private dayLabels = computed(() => {
        const labels: string[] = [];
        const startDate = this.from() ?? subDays(new Date(), this.days() - 1);
        let date = new Date(startDate);
        for (let i = 0; i < this.days(); i++) {
            labels.push(this.toDayKey(date));
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
            const dayKey = this.toDayKey(t.startAt);
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

        const datasets: TrackedTimeChartDataset[] = aggregates.map(agg => ({
            label: `#${agg.idIssuePublic} ${agg.issueTitle}`,
            data: labels.map(day => agg.hoursPerDay.get(day) ?? 0),
            borderWidth: 0,
            borderRadius: 2,
            barThickness: 26,
            idIssuePublic: agg.idIssuePublic,
            issueTitle: agg.issueTitle
        }));

        return { labels, datasets };
    });

    public chartOptions = computed<ChartOptions<'bar'>>(() => ({
        indexAxis: 'y',
        responsive: true,
        maintainAspectRatio: false,
        onHover: (e, els) => {
            (e.native?.target as HTMLElement).style.cursor = els.length ? 'pointer' : 'default';
        },
        layout: { padding: { right: 48 } },
        scales: {
            x: {
                stacked: true,
                beginAtZero: true,
                suggestedMax: 8,
                ticks: { callback: value => `${value}h` },
                title: { display: !!this.yAxisLabel(), text: this.yAxisLabel() }
            },
            y: { stacked: true, grid: { display: false } }
        },
        plugins: {
            legend: { display: false },
            tooltip: {
                callbacks: {
                    title: items => items[0]?.label ?? '',
                    label: ctx => {
                        const hours = ctx.parsed.x ?? 0;
                        return ` ${ctx.dataset.label}: ${TrackedTimeChartText.formatHours(hours)}`;
                    }
                }
            }
        }
    }));

    public heightPx = computed(() => this.days() * 36 + 48);

    public isEmpty = computed(() => this.issueAggregates().length === 0);

    protected onChartClick(event: { active?: object[] }): void {
        const active = event.active?.[0] as ActiveElement | undefined;
        if (active === undefined) {
            return;
        }
        const aggregate = this.issueAggregates()[active.datasetIndex];
        const dayKey = this.dayLabels()[active.index];
        if (aggregate === undefined || dayKey === undefined) {
            return;
        }
        this.selection.set({ idIssue: aggregate.idIssue, dayKey });
        const segmentTracks = TrackedTimeChartSelection.forSegment(
            this.tracks(),
            aggregate.idIssue,
            dayKey,
            this.toDayKey
        );
        this.editingId.set(segmentTracks.length === 1 ? segmentTracks[0].idTrack : null);
    }

    protected onTrackEditClose(): void {
        this.editingId.set(null);
        if (this.isSingle()) {
            this.selection.set(null);
        }
    }

    protected onDialogVisibleChange(visible: boolean): void {
        if (!visible) {
            this.selection.set(null);
            this.editingId.set(null);
        }
    }

    protected onDeleteTrack(track: Track): void {
        this.trackerService.deleteTrack$(track.idTrack).subscribe();
    }

    protected formatDuration(seconds: number): string {
        return DurationFormatter.durationToString(DurationConverter.secondsToDuration(seconds));
    }
}

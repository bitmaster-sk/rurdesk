import { ChangeDetectionStrategy, Component, computed, inject, input, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { DecimalPipe } from '@angular/common';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { Chart, ChartData, ChartOptions, LegendItem, Plugin } from 'chart.js';
import { DateUtil, utcDayRollover$ } from 'src/app/shared/date/date.util';
import { SprintChartMode } from '../../constants/sprint-chart-mode.enum';
import { SprintPhase } from '../../constants/sprint-phase.enum';
import { SprintUnit } from '../../constants/sprint-unit.enum';
import { SprintAnalyticsConverter } from '../../converter/sprint-analytics.converter';
import { SprintBurndown } from '../../model/sprint-burndown.model';
import { Sprint } from '../../model/sprint.model';
import { SprintStats } from '../../model/sprint-stats.model';
import { SprintVelocity } from '../../model/sprint-velocity.model';
import { SprintForecastFinish } from '../../entity/sprint-health.entity';
import { SprintHealthService } from '../../service/sprint-health.service';
import { SprintHealthText } from '../sprint-health-strip/sprint-health-strip.component';

interface SprintChartPalette {
    primary: string;
    muted: string;
    border: string;
    text: string;
    planned: string;
}

@Component({
    selector: 'app-sprint-charts-band',
    templateUrl: './sprint-charts-band.component.html',
    styleUrls: ['./sprint-charts-band.component.scss'],
    standalone: false,
    providers: [DecimalPipe],
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SprintChartsBandComponent {
    private readonly i18n = inject(I18nService);

    private readonly decimal = inject(DecimalPipe);

    private readonly healthService = inject(SprintHealthService);

    private readonly lang = toSignal(this.i18n.langChange$, { initialValue: null });

    private readonly clock = toSignal(utcDayRollover$, { initialValue: new Date() });

    public readonly sprint = input<Sprint | null>(null);

    public readonly projectName = input('');

    public readonly burndown = input<SprintBurndown | null>(null);

    public readonly stats = input<SprintStats | null>(null);

    public readonly velocities = input<SprintVelocity[]>([]);

    public readonly unit = input.required<SprintUnit>();

    public readonly isLoading = input(false);

    protected readonly mode = signal(SprintChartMode.Burndown);

    protected readonly modeOptions = [
        { labelKey: 'ISSUE.KANBAN.SPRINTS.BURNDOWN', value: SprintChartMode.Burndown },
        { labelKey: 'ISSUE.KANBAN.SPRINTS.BURNUP', value: SprintChartMode.Burnup }
    ];

    private readonly palette = SprintChartsBandComponent.readPalette();

    private readonly health = computed(() =>
        this.healthService.toHealth(
            this.sprint(),
            this.stats(),
            SprintAnalyticsConverter.toRecentAverages(this.velocities()),
            this.unit(),
            this.clock()
        )
    );

    private readonly effectiveUnit = computed(() =>
        this.sprint() === null
            ? this.healthService.forcedUnit(this.stats(), this.unit())
            : this.health().unit
    );

    private readonly series = computed(() =>
        SprintAnalyticsConverter.toBurndownSeries(this.burndown(), this.effectiveUnit())
    );

    private readonly ideal = computed(() =>
        SprintAnalyticsConverter.toIdealLine(this.burndown(), this.effectiveUnit())
    );

    private readonly scopeChange = computed(() =>
        SprintAnalyticsConverter.toScopeAdded(this.burndown(), this.effectiveUnit())
    );

    private readonly velocitySeries = computed(() =>
        SprintAnalyticsConverter.toVelocitySeries(this.velocities(), this.effectiveUnit())
    );

    private readonly windows = computed(() =>
        SprintAnalyticsConverter.toVelocityWindows(this.velocities(), this.effectiveUnit())
    );

    protected readonly header = computed(() => {
        this.lang();
        const sprint = this.sprint();
        if (sprint === null) {
            return this.i18n.instant('ISSUE.KANBAN.SPRINTS.VELOCITY_OF', {
                project: this.projectName()
            });
        }
        return `${sprint.name} · ${this.i18n.instant('ISSUE.KANBAN.SPRINTS.HEALTH_WINDOW', {
            start: this.formatDay(sprint.startAt),
            end: this.formatDay(sprint.endAt)
        })}`;
    });

    protected readonly hasBurndown = computed(() => this.series().labels.length > 0);

    private readonly todayIndex = computed(() => {
        const today = DateUtil.truncateTimeUtc(this.clock()).getTime();
        return (this.burndown()?.days ?? []).findIndex(
            day => DateUtil.truncateTimeUtc(day.day).getTime() === today
        );
    });

    private readonly lastRealIndex = computed(() => this.series().isReal.lastIndexOf(true));

    protected readonly hasVelocity = computed(() => this.velocities().length > 0);

    protected readonly scopeText = computed((): SprintHealthText => {
        const change = this.scopeChange();
        if (change.delta > 0) {
            return {
                key: 'ISSUE.KANBAN.SPRINTS.SCOPE_ADDED',
                params: { delta: change.delta, unit: this.unitLabel(change.delta) }
            };
        }
        if (change.delta < 0) {
            return {
                key: 'ISSUE.KANBAN.SPRINTS.SCOPE_REMOVED',
                params: { delta: Math.abs(change.delta), unit: this.unitLabel(change.delta) }
            };
        }
        return { key: 'ISSUE.KANBAN.SPRINTS.SCOPE_UNCHANGED' };
    });

    protected readonly scopeDetail = computed((): SprintHealthText | null => {
        const change = this.scopeChange();
        if (change.day === null || change.dayIndex === null) {
            return null;
        }
        if (change.dayIndex > change.daysTotal) {
            return {
                key: 'ISSUE.KANBAN.SPRINTS.SCOPE_CHANGE_DETAIL_LATE',
                params: { date: this.formatDay(change.day) }
            };
        }
        return {
            key: 'ISSUE.KANBAN.SPRINTS.SCOPE_ADDED_DETAIL',
            params: {
                date: this.formatDay(change.day),
                day: change.dayIndex,
                days: change.daysTotal
            }
        };
    });

    private readonly forecast = computed((): SprintForecastFinish | null => {
        const sprint = this.sprint();
        const health = this.health();
        if (sprint === null || health.phase !== SprintPhase.Running) {
            return null;
        }
        return this.healthService.toForecastFinish(sprint, health);
    });

    protected readonly forecastText = computed((): SprintHealthText | null => {
        const forecast = this.forecast();
        return forecast === null
            ? null
            : {
                  key: 'ISSUE.KANBAN.SPRINTS.FORECAST_FINISH',
                  params: { date: this.formatDay(forecast.finish.toISOString()) }
              };
    });

    protected readonly forecastDetail = computed((): SprintHealthText | null => {
        const forecast = this.forecast();
        return forecast === null || forecast.daysLate === 0
            ? null
            : {
                  key: this.pluralKey('ISSUE.KANBAN.SPRINTS.FORECAST_LATE', forecast.daysLate),
                  params: { count: forecast.daysLate }
              };
    });

    protected readonly avgText = computed((): SprintHealthText | null => {
        const recent = this.windows().recent;
        if (recent === null) {
            return null;
        }
        return {
            key: 'ISSUE.KANBAN.SPRINTS.AVG_VELOCITY',
            params: {
                avg: this.decimal.transform(recent, '1.0-1') ?? '0',
                unit: this.unitLabel(recent)
            }
        };
    });

    protected readonly avgTrend = computed((): SprintHealthText | null => {
        const windows = this.windows();
        if (windows.previous === null || windows.recent === null || windows.previousCount < 2) {
            return null;
        }
        const key =
            windows.recent >= windows.previous
                ? 'ISSUE.KANBAN.SPRINTS.AVG_VELOCITY_TREND_UP'
                : 'ISSUE.KANBAN.SPRINTS.AVG_VELOCITY_TREND_DOWN';
        return {
            key: this.pluralKey(key, windows.previousCount),
            params: {
                avg: this.decimal.transform(windows.previous, '1.0-1') ?? '0',
                count: windows.previousCount
            }
        };
    });

    protected readonly hasLiveFallback = computed(() =>
        this.velocities().some(entry => !entry.frozen)
    );

    protected readonly burndownData = computed((): ChartData<'line'> => {
        this.lang();
        const series = this.series();
        const isBurnup = this.mode() === SprintChartMode.Burnup;
        const primary = isBurnup ? series.done : series.remaining;
        const secondary = isBurnup ? series.scope : this.ideal();
        return {
            labels: series.labels.map(label => this.formatDay(label)),
            datasets: [
                {
                    label: this.i18n.instant(
                        isBurnup
                            ? 'ISSUE.KANBAN.SPRINTS.COMPLETED'
                            : 'ISSUE.KANBAN.SPRINTS.REMAINING'
                    ),
                    data: primary,
                    borderColor: this.palette.primary,
                    backgroundColor: this.palette.primary,
                    tension: 0.15,
                    spanGaps: false,
                    pointRadius: ctx => (series.isReal[ctx.dataIndex] ? 3 : 0),
                    segment: {
                        borderDash: ctx => (series.isReal[ctx.p1DataIndex] ? undefined : [4, 4])
                    }
                },
                {
                    label: this.i18n.instant(
                        isBurnup ? 'ISSUE.KANBAN.SPRINTS.SCOPE' : 'ISSUE.KANBAN.SPRINTS.IDEAL'
                    ),
                    data: secondary,
                    borderColor: this.palette.muted,
                    backgroundColor: this.palette.muted,
                    borderDash: [6, 6],
                    pointRadius: 0,
                    spanGaps: false
                }
            ]
        };
    });

    protected readonly velocityData = computed((): ChartData<'bar'> => {
        this.lang();
        const series = this.velocitySeries();
        const recent = this.windows().recent;
        return {
            labels: series.labels,
            datasets: [
                {
                    label: this.i18n.instant('ISSUE.KANBAN.SPRINTS.COMPLETED'),
                    data: series.done,
                    backgroundColor: this.palette.primary,
                    borderWidth: 0,
                    borderRadius: 4
                },
                {
                    label: this.i18n.instant('ISSUE.KANBAN.SPRINTS.PLANNED'),
                    data: series.planned,
                    backgroundColor: this.palette.planned,
                    borderWidth: 0,
                    borderRadius: 4
                },
                ...(recent
                    ? [
                          {
                              type: 'line',
                              label: this.i18n.instant('ISSUE.KANBAN.SPRINTS.AVG_VELOCITY_LEGEND'),
                              data: series.labels.map(() => recent),
                              borderColor: this.palette.muted,
                              borderDash: [4, 4],
                              borderWidth: 1,
                              pointRadius: 0
                          }
                      ]
                    : [])
            ] as ChartData<'bar'>['datasets']
        };
    });

    protected readonly chartOptions: ChartOptions<'line'> = {
        responsive: true,
        maintainAspectRatio: false,
        animation: false,
        layout: { padding: { top: 10 } },
        interaction: { mode: 'index', intersect: false },
        scales: {
            x: { grid: { display: false } },
            y: { beginAtZero: true, grace: '8%', grid: { display: false } }
        },
        plugins: {
            legend: {
                position: 'bottom',
                labels: {
                    usePointStyle: true,
                    generateLabels: chart => SprintChartsBandComponent.legendItems(chart)
                }
            }
        }
    };

    protected readonly velocityOptions: ChartOptions<'bar'> = {
        responsive: true,
        maintainAspectRatio: false,
        animation: false,
        layout: { padding: { top: 10 } },
        scales: {
            x: { grid: { display: false } },
            y: { beginAtZero: true, grace: '8%', grid: { display: false } }
        },
        plugins: {
            legend: {
                position: 'bottom',
                labels: {
                    usePointStyle: true,
                    generateLabels: chart => SprintChartsBandComponent.legendItems(chart)
                }
            }
        }
    };

    protected readonly burndownPlugins: Plugin<'line'>[] = [
        {
            id: 'sprintBurndownMarkers',
            afterDatasetsDraw: chart => this.drawTodayMarker(chart)
        }
    ];

    protected readonly velocityPlugins: Plugin<'bar'>[] = [
        {
            id: 'sprintVelocityLabels',
            afterDatasetsDraw: chart => this.drawBarValues(chart)
        }
    ];

    private static token(name: string, fallback: string): string {
        const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
        return value || fallback;
    }

    private static readPalette(): SprintChartPalette {
        return {
            primary: SprintChartsBandComponent.token('--ui-color-primary', '#2563eb'),
            muted: SprintChartsBandComponent.token('--ui-color-text-muted', '#6b7280'),
            border: SprintChartsBandComponent.token('--ui-color-border', '#d1d5db'),
            text: SprintChartsBandComponent.token('--ui-color-text', '#111827'),
            planned: SprintChartsBandComponent.token('--ui-sky-700', '#0369a1')
        };
    }

    private static legendItems(chart: Chart): LegendItem[] {
        return chart.data.datasets.map((dataset, index) => {
            const kind = dataset.type ?? (chart.config as { type?: string }).type;
            const isBar = kind === 'bar';
            const color = (isBar ? dataset.backgroundColor : dataset.borderColor) as string;
            return {
                text: dataset.label ?? '',
                fillStyle: color,
                strokeStyle: color,
                lineWidth: isBar ? 0 : 2,
                lineDash: (dataset as { borderDash?: number[] }).borderDash ?? [],
                pointStyle: isBar ? 'rect' : 'line',
                hidden: !chart.isDatasetVisible(index),
                datasetIndex: index
            };
        });
    }

    protected onModeChange(mode: SprintChartMode): void {
        this.mode.set(mode);
    }

    private drawTodayMarker(chart: Chart): void {
        const { ctx, chartArea } = chart;
        const meta = chart.getDatasetMeta(0);
        const today = meta.data[this.todayIndex()];
        ctx.save();
        if (today) {
            ctx.strokeStyle = this.palette.border;
            ctx.setLineDash([2, 3]);
            ctx.beginPath();
            ctx.moveTo(today.x, chartArea.top);
            ctx.lineTo(today.x, chartArea.bottom);
            ctx.stroke();
            ctx.setLineDash([]);
        }

        const lastReal = this.lastRealIndex();
        const point = meta.data[lastReal];
        const value = chart.data.datasets[0].data[lastReal];
        if (point && typeof value === 'number') {
            ctx.fillStyle = this.palette.text;
            ctx.font = '11px sans-serif';
            ctx.textAlign = 'right';
            ctx.fillText(String(value), point.x - 4, chartArea.top + 12);
        }
        ctx.restore();
    }

    private drawBarValues(chart: Chart): void {
        const { ctx } = chart;
        ctx.save();
        ctx.fillStyle = this.palette.text;
        ctx.font = '11px sans-serif';
        ctx.textAlign = 'center';
        chart.data.datasets.forEach((dataset, datasetIndex) => {
            if (dataset.type === 'line') {
                return;
            }
            chart.getDatasetMeta(datasetIndex).data.forEach((bar, index) => {
                const value = dataset.data[index];
                if (typeof value !== 'number') {
                    return;
                }
                ctx.fillText(String(value), bar.x, bar.y - 4);
            });
        });
        ctx.restore();
    }

    private pluralKey(key: string, count: number): string {
        return `${key}.${Math.abs(count) === 1 ? 'SINGULAR' : 'PLURAL'}`;
    }

    private unitLabel(count: number): string {
        this.lang();
        const key =
            this.effectiveUnit() === SprintUnit.Points
                ? 'ISSUE.KANBAN.SPRINTS.UNIT_POINTS_SHORT'
                : 'ISSUE.KANBAN.SPRINTS.UNIT_TASKS_SHORT';
        return this.i18n.instant(`${key}.${Math.abs(count) === 1 ? 'SINGULAR' : 'PLURAL'}`);
    }

    private formatDay(value: string): string {
        return new Date(value).toLocaleDateString(this.i18n.currentLang, {
            timeZone: 'UTC',
            month: 'short',
            day: 'numeric'
        });
    }
}

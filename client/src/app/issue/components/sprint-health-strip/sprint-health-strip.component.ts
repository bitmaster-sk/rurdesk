import { ChangeDetectionStrategy, Component, computed, inject, input, model } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { DecimalPipe } from '@angular/common';
import { TranslateService } from '@ngx-translate/core';
import { defer, timer } from 'rxjs';
import { map, repeat } from 'rxjs/operators';
import { DAY_MS, DateUtil, msUntilNextUtcDay } from 'src/app/shared/date/date.util';
import { SprintPhase } from '../../constants/sprint-phase.enum';
import { SprintUnit } from '../../constants/sprint-unit.enum';
import { SprintHealth, SprintHealthForecast } from '../../entity/sprint-health.entity';
import { Sprint } from '../../model/sprint.model';
import { SprintStats } from '../../model/sprint-stats.model';
import { SprintVelocity, SprintVelocityAverages } from '../../model/sprint-velocity.model';
import { SprintHealthService } from '../../service/sprint-health.service';
import { UiTagSeverity } from 'src/app/ui/components/tag/tag.component';

export interface SprintHealthText {
    key: string;
    params?: Record<string, string | number>;
}

export interface SprintHealthChip extends SprintHealthText {
    severity: UiTagSeverity;
    icon: string | null;
}

const utcDayRollover$ = defer(() => timer(msUntilNextUtcDay(new Date()))).pipe(
    repeat(),
    map(() => new Date())
);

@Component({
    selector: 'app-sprint-health-strip',
    templateUrl: './sprint-health-strip.component.html',
    styleUrls: ['./sprint-health-strip.component.scss'],
    standalone: false,
    providers: [DecimalPipe],
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SprintHealthStripComponent {
    private readonly healthService = inject(SprintHealthService);

    private readonly i18n = inject(TranslateService);

    private readonly decimal = inject(DecimalPipe);

    private readonly lang = toSignal(this.i18n.onLangChange, { initialValue: null });

    private readonly clock = toSignal(utcDayRollover$, { initialValue: new Date() });

    public readonly sprint = input<Sprint | null>(null);

    public readonly stats = input<SprintStats | null>(null);

    public readonly velocities = input<SprintVelocity[]>([]);

    public readonly unit = model.required<SprintUnit>();

    protected readonly sprintPhase = SprintPhase;

    protected readonly unitOptions = [
        { labelKey: 'ISSUE.KANBAN.SPRINTS.UNIT_POINTS', value: SprintUnit.Points },
        { labelKey: 'ISSUE.KANBAN.SPRINTS.UNIT_TASKS', value: SprintUnit.Issues }
    ];

    protected readonly avgVelocity = computed<SprintVelocityAverages | null>(() => {
        const recent = this.velocities();
        if (recent.length === 0) {
            return null;
        }
        return {
            points: this.averageOfScored(recent.map(v => v.donePoints)),
            issues: this.averageOfScored(recent.map(v => v.doneIssues))
        };
    });

    protected readonly health = computed<SprintHealth>(() =>
        this.healthService.toHealth(
            this.sprint(),
            this.stats(),
            this.avgVelocity(),
            this.unit(),
            this.clock()
        )
    );

    protected readonly segments = computed(() => {
        const health = this.health();
        const total = health.committed;
        if (total <= 0) {
            return { done: 0, progress: 0, todo: 0 };
        }
        return {
            done: (health.done / total) * 100,
            progress: (health.inProgress / total) * 100,
            todo: (health.notStarted / total) * 100
        };
    });

    protected readonly window = computed(() => {
        const sprint = this.sprint();
        if (sprint === null) {
            return null;
        }
        this.lang();
        return this.i18n.instant('ISSUE.KANBAN.SPRINTS.HEALTH_WINDOW', {
            start: this.formatDay(sprint.startAt),
            end: this.formatDay(sprint.endAt)
        });
    });

    protected readonly progressText = computed((): SprintHealthText => {
        const health = this.health();
        if (this.sprint() === null) {
            const stats = this.stats();
            const tasks = (stats?.totalIssues ?? 0) - (stats?.doneIssues ?? 0);
            const points = (stats?.totalPoints ?? 0) - (stats?.donePoints ?? 0);
            return {
                key: 'ISSUE.KANBAN.SPRINTS.HEALTH_BACKLOG_SUMMARY',
                params: {
                    tasks,
                    points,
                    taskUnit: this.shortUnit(SprintUnit.Issues, tasks),
                    pointUnit: this.shortUnit(SprintUnit.Points, points)
                }
            };
        }
        if (health.phase === SprintPhase.Closed) {
            return {
                key: 'ISSUE.KANBAN.SPRINTS.HEALTH_DONE_ONLY',
                params: { done: health.done, unit: this.unitLabel(health.done) }
            };
        }
        if (health.phase === SprintPhase.NotStarted) {
            return {
                key: 'ISSUE.KANBAN.SPRINTS.HEALTH_PLANNED',
                params: { count: health.committed, unit: this.unitLabel(health.committed) }
            };
        }
        return {
            key: 'ISSUE.KANBAN.SPRINTS.HEALTH_PROGRESS',
            params: {
                done: health.done,
                total: health.committed,
                unit: this.unitLabel(health.committed)
            }
        };
    });

    protected readonly forecast = computed((): SprintHealthForecast | null => {
        const health = this.health();
        return health.phase === SprintPhase.Running ? health.forecast : null;
    });

    protected readonly timingText = computed((): SprintHealthText | null => {
        const sprint = this.sprint();
        const health = this.health();
        if (sprint === null) {
            return null;
        }
        if (health.phase === SprintPhase.NotStarted) {
            const days =
                (DateUtil.truncateTimeUtc(sprint.startAt).getTime() -
                    DateUtil.truncateTimeUtc(this.clock()).getTime()) /
                DAY_MS;
            const count = Math.max(days, 0);
            return {
                key: this.pluralKey('ISSUE.KANBAN.SPRINTS.HEALTH_STARTS_IN', count),
                params: { count }
            };
        }
        if (health.phase !== SprintPhase.Running) {
            return null;
        }
        return health.daysLeft > 0
            ? {
                  key: this.pluralKey('ISSUE.KANBAN.SPRINTS.HEALTH_DAYS_LEFT', health.daysLeft),
                  params: { count: health.daysLeft }
              }
            : {
                  key: 'ISSUE.KANBAN.SPRINTS.HEALTH_DAY_X_OF_Y',
                  params: { day: health.daysElapsed, days: health.daysTotal }
              };
    });

    protected readonly verdictChip = computed((): SprintHealthChip | null => {
        const health = this.health();
        if (health.phase === SprintPhase.NotStarted) {
            return health.isOverCommitted
                ? {
                      key: 'ISSUE.KANBAN.SPRINTS.HEALTH_OVER_COMMITTED',
                      params: {
                          avg: this.decimal.transform(health.avgVelocity ?? 0, '1.0-1') ?? '0'
                      },
                      severity: 'warn',
                      icon: 'alert-triangle'
                  }
                : null;
        }
        if (health.phase !== SprintPhase.Running) {
            return null;
        }
        if (health.isTooEarly) {
            return {
                key: 'ISSUE.KANBAN.SPRINTS.HEALTH_TOO_EARLY',
                severity: 'secondary',
                icon: null
            };
        }
        if (health.forecast === null) {
            return null;
        }
        return health.forecast.isOnTrack
            ? {
                  key: 'ISSUE.KANBAN.SPRINTS.HEALTH_ON_TRACK',
                  severity: 'success',
                  icon: 'check'
              }
            : {
                  key: 'ISSUE.KANBAN.SPRINTS.HEALTH_BEHIND',
                  params: {
                      gap: health.forecast.gap,
                      unit: this.unitLabel(health.forecast.gap)
                  },
                  severity: 'danger',
                  icon: 'alert-triangle'
              };
    });

    protected onUnitChange(unit: SprintUnit): void {
        this.unit.set(unit);
    }

    private unitLabel(count: number): string {
        return this.shortUnit(this.health().unit, count);
    }

    private shortUnit(unit: SprintUnit, count: number): string {
        this.lang();
        const key =
            unit === SprintUnit.Points
                ? 'ISSUE.KANBAN.SPRINTS.UNIT_POINTS_SHORT'
                : 'ISSUE.KANBAN.SPRINTS.UNIT_TASKS_SHORT';
        return this.i18n.instant(this.pluralKey(key, count));
    }

    private pluralKey(key: string, count: number): string {
        return `${key}.${count === 1 ? 'SINGULAR' : 'PLURAL'}`;
    }

    private formatDay(value: string): string {
        return new Date(value).toLocaleDateString(this.i18n.currentLang || undefined, {
            timeZone: 'UTC',
            month: 'short',
            day: 'numeric'
        });
    }

    private averageOfScored(values: number[]): number {
        const scored = values.filter(value => value > 0);
        return scored.length === 0
            ? 0
            : scored.reduce((sum, value) => sum + value, 0) / scored.length;
    }
}

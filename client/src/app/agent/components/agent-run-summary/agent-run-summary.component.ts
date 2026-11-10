import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { RunStats } from '../../model/agent-run.model';
import { AgentEffort } from '../../model/agent-effort.enum';
import { CostFormatter } from '../../entity/cost.formatter';
import { DurationFormatter } from 'src/app/shared/duration/duration.formatter';

@Component({
    selector: 'app-agent-run-summary',
    templateUrl: './agent-run-summary.component.html',
    styleUrls: ['./agent-run-summary.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AgentRunSummaryComponent {
    public readonly model = input<string | null>(null);
    public readonly effort = input<AgentEffort | null>(null);
    public readonly startedAt = input<string | null>(null);
    public readonly stats = input<RunStats | null>(null);
    public readonly isShortWorkflow = input(false);
    public readonly canChangeWorkflow = input(false);
    public readonly isWorkflowLockShown = input(false);
    public readonly isSavingWorkflow = input(false);

    public readonly workflowToggled = output<Event>();

    protected readonly duration = computed(() => {
        const stats = this.stats();
        return stats ? DurationFormatter.millisecondsToString(stats.totalDurationMs) : null;
    });

    protected readonly modelRows = computed(() => {
        const rows = Object.entries(this.stats()?.tokensByModel ?? {})
            .map(([model, tokens]) => ({ model, tokens }))
            .sort((left, right) => right.tokens - left.tokens);
        return rows.length > 1 ? rows : [];
    });

    protected readonly cost = computed(() => {
        const stats = this.stats();
        if (!stats) {
            return null;
        }
        return CostFormatter.format(stats.totalCostUsd, stats.costEstimated);
    });

    protected readonly tokenBreakdown = computed(() => {
        const stats = this.stats();
        if (!stats) {
            return null;
        }
        const parts = [
            stats.totalInputTokens,
            stats.totalOutputTokens,
            stats.totalCacheReadTokens,
            stats.totalCacheWriteTokens
        ];
        return parts.some(value => value > 0) ? stats : null;
    });
}

import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    OnInit,
    computed,
    inject,
    input,
    signal
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { AgentRunApi } from '../../api/agent-run.api.service';
import { RunStats } from '../../model/agent-run.model';
import { STAGE_LABELS } from '../../model/agent-stage.enum';
import { NoticeService } from 'src/app/shared/notice/notice.service';

@Component({
    selector: 'app-run-stats-panel',
    templateUrl: './run-stats-panel.component.html',
    styleUrls: ['./run-stats-panel.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class RunStatsPanelComponent implements OnInit {
    public readonly idRun = input.required<number>();

    private readonly api = inject(AgentRunApi);
    private readonly noticeService = inject(NoticeService);
    private readonly destroyRef = inject(DestroyRef);

    protected readonly stats = signal<RunStats | null>(null);
    protected readonly stageLabel = STAGE_LABELS as Record<string, string>;

    protected readonly stageRows = computed(() => {
        const s = this.stats();
        if (s === null) {
            return [];
        }
        return Object.entries(s.attemptsPerStage).map(([stage, count]) => ({
            stage,
            label: this.stageLabel[stage] ?? stage,
            count
        }));
    });

    public ngOnInit(): void {
        this.api.stats$(this.idRun()).subscribe(s => this.stats.set(s));
        this.noticeService.agentStats$
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(notice => {
                const payload = notice.payload as { idRun: number; stats: RunStats } | null;
                if (payload && payload.idRun === this.idRun()) {
                    this.stats.set(payload.stats);
                }
            });
    }

    protected formatDuration(ms: number): string {
        const sec = Math.round(ms / 1000);
        if (sec < 60) return `${sec}s`;
        return `${Math.floor(sec / 60)}m ${sec % 60}s`;
    }
}

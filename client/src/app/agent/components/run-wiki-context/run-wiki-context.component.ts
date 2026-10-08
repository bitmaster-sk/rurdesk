import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    computed,
    effect,
    inject,
    input,
    signal,
    untracked
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Subject, catchError, filter, of, switchMap } from 'rxjs';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { AgentRunApi } from '../../api/agent-run.api.service';
import { RunWikiContextConverter } from '../../converter/run-wiki-context.converter';
import { RunWikiStage } from '../../entity/run-wiki-context.entity';
import { AgentRun } from '../../model/agent-run.model';
import { AgentRunWikiRead } from '../../model/agent-run-wiki-read.model';
import { STAGE_LABELS } from '../../model/agent-stage.enum';

@Component({
    selector: 'app-run-wiki-context',
    templateUrl: './run-wiki-context.component.html',
    styleUrls: ['./run-wiki-context.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class RunWikiContextComponent {
    public readonly run = input.required<AgentRun>();

    private readonly api = inject(AgentRunApi);
    private readonly noticeService = inject(NoticeService);
    private readonly destroyRef = inject(DestroyRef);

    private readonly reads = signal<AgentRunWikiRead[]>([]);
    private readonly openIndexes = signal<ReadonlySet<string>>(new Set());
    private readonly reload$ = new Subject<number>();

    private readonly stageLabels: Partial<Record<string, string>> = STAGE_LABELS;

    protected readonly stages = computed<RunWikiStage[]>(() => {
        const run = this.run();
        return RunWikiContextConverter.toStages(
            this.reads(),
            run.idProject,
            (run.stagePlan?.stages ?? []).map(stage => stage.name)
        );
    });

    private readonly reloadKey = computed(() => {
        const run = this.run();
        const stages = (run.stages ?? [])
            .map(stage => `${stage.stage}:${stage.status}:${stage.attemptNo ?? 0}`)
            .join(',');
        return `${run.idRun}|${run.phase}|${stages}`;
    });

    public constructor() {
        this.reload$
            .pipe(
                switchMap(idRun => this.api.loadWikiReads$(idRun).pipe(catchError(() => of(null)))),
                filter((reads): reads is AgentRunWikiRead[] => reads !== null),
                takeUntilDestroyed(this.destroyRef)
            )
            .subscribe(reads => this.reads.set(reads));

        this.noticeService.agentStats$
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(notice => {
                const payload = notice.payload as { idRun: number } | null;
                if (payload?.idRun === this.run().idRun) {
                    this.reload$.next(payload.idRun);
                }
            });

        effect(() => {
            this.reloadKey();
            const idRun = untracked(() => this.run().idRun);
            this.reload$.next(idRun);
        });
    }

    protected isIndexOpen(stage: string): boolean {
        return this.openIndexes().has(stage);
    }

    protected onToggleIndex(stage: string): void {
        this.openIndexes.update(open => {
            const next = new Set(open);
            if (!next.delete(stage)) {
                next.add(stage);
            }
            return next;
        });
    }

    protected stageLabel(stage: string): string {
        return this.stageLabels[stage] ?? stage;
    }
}

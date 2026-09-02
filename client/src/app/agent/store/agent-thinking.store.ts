import { DestroyRef, Injectable, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { THINKING_BUFFER_CHARS } from '../constants/agent-thinking.constants';
import { AgentThinkingConverter } from '../converter/agent-thinking.converter';
import { AgentThinkingLine } from '../entity/agent-thinking.entity';
import { AgentThinkingEvent, AgentThinkingNotice } from '../model/agent-thinking.model';

@Injectable()
export class AgentThinkingStore {
    private readonly noticeService = inject(NoticeService);
    private readonly destroyRef = inject(DestroyRef);

    public readonly events = signal<AgentThinkingEvent[]>([]);
    public readonly idTask = signal<number | null>(null);
    public readonly stage = signal<string | null>(null);
    public readonly hasGap = signal(false);

    public readonly lines = computed<AgentThinkingLine[]>(() =>
        AgentThinkingConverter.toLines(this.events())
    );

    private idRun: number | null = null;
    private lastSeq = 0;
    private firstSeq = 0;
    private appliedStage: string | null = null;

    public constructor() {
        this.noticeService.agentThinking$
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(notice => this.apply(notice.payload as AgentThinkingNotice | null));
    }

    public bind(idRun: number | null): void {
        if (this.idRun === idRun) {
            return;
        }
        this.idRun = idRun;
        this.reset();
    }

    // applyStored puts the thinking the server already has in front of whatever
    // the stream has delivered so far, so a reloaded page rejoins a running stage
    // instead of reporting everything before it as lost.
    public applyStored(
        stage: string,
        idTask: number,
        events: AgentThinkingEvent[],
        lastSeq: number
    ): void {
        if (this.idRun === null || lastSeq === 0 || this.appliedStage === stage) {
            return;
        }
        const streamedStage = this.stage();
        if (streamedStage !== null && (streamedStage !== stage || this.idTask() !== idTask)) {
            return;
        }
        this.appliedStage = stage;

        const merged = [...events, ...this.events()];
        const kept = this.capped(merged);
        this.events.set(kept);

        if (this.lastSeq === 0) {
            this.idTask.set(idTask);
            this.stage.set(stage);
            this.lastSeq = lastSeq;
            this.hasGap.set(kept.length < merged.length);
            return;
        }
        this.hasGap.set(kept.length < merged.length || this.firstSeq > lastSeq + 1);
    }

    private apply(notice: AgentThinkingNotice | null): void {
        if (!notice || this.idRun === null || notice.idRun !== this.idRun) {
            return;
        }
        // A retried stage keeps its name but gets a new task, and seq restarts at
        // 1 for it, so the stage name alone cannot separate the attempts.
        if (notice.idTask !== this.idTask() || notice.stage !== this.stage()) {
            this.reset();
            this.idTask.set(notice.idTask);
            this.stage.set(notice.stage);
            if (notice.seq > 1) {
                this.hasGap.set(true);
            }
        } else if (notice.seq <= this.lastSeq) {
            // The gateway resends a failed batch under its original seq, so the
            // same events can arrive twice.
            return;
        } else if (notice.seq > this.lastSeq + 1) {
            this.hasGap.set(true);
        }
        this.lastSeq = notice.seq;
        if (this.firstSeq === 0) {
            this.firstSeq = notice.seq;
        }

        const merged = [...this.events(), ...notice.events];
        const kept = this.capped(merged);
        if (kept.length < merged.length) {
            this.hasGap.set(true);
        }
        this.events.set(kept);
    }

    private capped(events: AgentThinkingEvent[]): AgentThinkingEvent[] {
        let total = events.reduce((sum, event) => sum + this.lengthOf(event), 0);
        let firstKept = 0;
        while (total > THINKING_BUFFER_CHARS && firstKept < events.length - 1) {
            total -= this.lengthOf(events[firstKept]);
            firstKept++;
        }
        return firstKept === 0 ? events : events.slice(firstKept);
    }

    private lengthOf(event: AgentThinkingEvent): number {
        return (event.text ?? '').length + (event.tool ?? '').length;
    }

    private reset(): void {
        this.events.set([]);
        this.idTask.set(null);
        this.stage.set(null);
        this.hasGap.set(false);
        this.lastSeq = 0;
        this.firstSeq = 0;
        this.appliedStage = null;
    }
}

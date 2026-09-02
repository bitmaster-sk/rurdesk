import {
    ChangeDetectionStrategy,
    Component,
    ElementRef,
    OnDestroy,
    computed,
    effect,
    inject,
    input,
    signal,
    viewChild
} from '@angular/core';
import { AgentThinkingApi } from 'src/app/agent/api/agent-thinking.api.service';
import { AgentThinkingConverter } from 'src/app/agent/converter/agent-thinking.converter';
import { AgentStageProgress } from 'src/app/agent/model/agent-run.model';
import { AgentStage, STAGE_LABELS } from 'src/app/agent/model/agent-stage.enum';
import { AgentThinkingKind } from 'src/app/agent/constants/agent-thinking-kind.enum';
import { AgentToolKind } from 'src/app/agent/constants/agent-tool-kind.enum';
import { AgentThinkingEvent } from 'src/app/agent/model/agent-thinking.model';
import { AgentThinkingStore } from 'src/app/agent/store/agent-thinking.store';
import { User } from 'src/app/auth/model/user.model';
import { DurationConverter } from 'src/app/shared/duration/duration.converter';
import { DurationFormatter } from 'src/app/shared/duration/duration.formatter';

@Component({
    selector: 'app-agent-thinking-row',
    templateUrl: './agent-thinking-row.component.html',
    styleUrls: ['./agent-thinking-row.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class AgentThinkingRowComponent implements OnDestroy {
    public readonly idRun = input.required<number>();
    public readonly stage = input.required<AgentStageProgress>();
    public readonly isLive = input(false);
    public readonly creator = input<User | null>(null);

    private readonly thinkingApi = inject(AgentThinkingApi);
    private readonly store = inject(AgentThinkingStore);

    protected readonly thinkingKind = AgentThinkingKind;

    private readonly body = viewChild<ElementRef<HTMLDivElement>>('body');

    private readonly isOpenOverride = signal<boolean | null>(null);
    private readonly storedEvents = signal<AgentThinkingEvent[]>([]);
    // Must go back to null on a failed fetch, or the row can never retry.
    private loadedAsTail: boolean | null = null;
    private readonly now = signal(Date.now());
    private ticker: ReturnType<typeof setInterval> | null = null;

    protected readonly isOpen = computed(() => this.isOpenOverride() ?? this.isLive());

    // The live row survives a stage switch and the store holds the finished stage
    // until the next one streams, so a named row must show only its own stage.
    protected readonly lines = computed(() => {
        if (!this.isLive()) {
            return AgentThinkingConverter.toLines(this.storedEvents());
        }
        const streaming = this.store.stage();
        const own = this.stage().stage;
        return streaming === null || own === '' || streaming === own ? this.store.lines() : [];
    });

    protected readonly hasGap = computed(() => this.isLive() && this.store.hasGap());

    protected readonly stageLabel = computed(
        () => STAGE_LABELS[this.stage().stage as AgentStage] ?? ''
    );

    protected readonly isTailOnly = computed(() => !this.isLive() && !this.stage().hasThinking);

    protected readonly elapsed = computed(() => {
        const startedAt = this.startedAt();
        if (!this.isLive() || startedAt === null) {
            return '';
        }
        const seconds = Math.max(0, Math.floor((this.now() - startedAt) / 1000));
        return DurationFormatter.durationToString(DurationConverter.secondsToDuration(seconds));
    });

    public constructor() {
        effect(() => {
            if (this.isLive() && this.ticker === null) {
                this.ticker = setInterval(() => this.now.set(Date.now()), 1000);
            }
        });
        effect(() => {
            this.lines();
            if (this.isLive() && this.isOpen()) {
                this.scrollToLatest();
            }
        });
    }

    public ngOnDestroy(): void {
        if (this.ticker !== null) {
            clearInterval(this.ticker);
        }
    }

    // Every kind must resolve to an icon registered in the module, or an unmapped
    // tool renders an empty box.
    protected toolIcon(kind: AgentToolKind): string {
        switch (kind) {
            case AgentToolKind.Run:
                return 'terminal-2';
            case AgentToolKind.Write:
                return 'pencil';
            case AgentToolKind.Read:
                return 'search';
            default:
                return 'hammer';
        }
    }

    protected onToggle(): void {
        const willOpen = !this.isOpen();
        this.isOpenOverride.set(willOpen);
        const isTail = this.isTailOnly();
        if (!willOpen || this.isLive() || this.loadedAsTail === isTail) {
            return;
        }
        this.loadedAsTail = isTail;
        if (isTail) {
            this.storedEvents.set(
                AgentThinkingConverter.toTailEvents(this.stage().thinkingTail ?? '')
            );
            return;
        }
        this.thinkingApi.loadStageThinking$(this.idRun(), this.stage().stage).subscribe({
            next: res => this.storedEvents.set(res.events),
            error: () => (this.loadedAsTail = null)
        });
    }

    private scrollToLatest(): void {
        setTimeout(() => {
            const element = this.body()?.nativeElement;
            if (element) {
                element.scrollTop = element.scrollHeight;
            }
        }, 0);
    }

    private startedAt(): number | null {
        const at = this.stage().at;
        return at ? new Date(at).getTime() : null;
    }
}

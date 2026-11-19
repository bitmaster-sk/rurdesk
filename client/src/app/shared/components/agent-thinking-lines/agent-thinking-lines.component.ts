import {
    ChangeDetectionStrategy,
    Component,
    ElementRef,
    effect,
    input,
    viewChild
} from '@angular/core';
import { AgentThinkingKind } from 'src/app/agent/constants/agent-thinking-kind.enum';
import { AgentToolKind } from 'src/app/agent/constants/agent-tool-kind.enum';
import { AgentThinkingLine } from 'src/app/agent/entity/agent-thinking.entity';

/** The agent's thoughts and tool calls of one stage or chat turn. */
@Component({
    selector: 'app-agent-thinking-lines',
    templateUrl: './agent-thinking-lines.component.html',
    styleUrls: ['./agent-thinking-lines.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class AgentThinkingLinesComponent {
    public readonly lines = input.required<AgentThinkingLine[]>();
    public readonly hasGap = input(false);
    /** A live list keeps its full height and follows the newest line. */
    public readonly isLive = input(false);

    protected readonly thinkingKind = AgentThinkingKind;

    private readonly body = viewChild<ElementRef<HTMLDivElement>>('body');

    public constructor() {
        effect(() => {
            this.lines();
            if (this.isLive()) {
                this.scrollToLatest();
            }
        });
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

    private scrollToLatest(): void {
        setTimeout(() => {
            const element = this.body()?.nativeElement;
            if (element) {
                element.scrollTop = element.scrollHeight;
            }
        }, 0);
    }
}

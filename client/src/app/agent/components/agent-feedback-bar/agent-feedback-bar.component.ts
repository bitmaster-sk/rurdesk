import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { AgentRun } from '../../model/agent-run.model';
import { AgentFeedback } from '../../entity/agent-feedback.entity';
import { AgentFeedbackConverter } from '../../converter/agent-feedback.converter';
import { AgentFeedbackAction } from '../../constants/agent-feedback-action.enum';

/** One row above the comment editor: what the agent waits for, how much is new, and Send or Approve. */
@Component({
    selector: 'app-agent-feedback-bar',
    templateUrl: './agent-feedback-bar.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AgentFeedbackBarComponent {
    public readonly run = input<AgentRun | null>(null);
    public readonly feedback = input<AgentFeedback>({ comments: 0, replies: 0, threads: 0 });
    public readonly agentName = input<string | null>(null);
    /** The design waiting for review offers mockups, which are approved on their own cards. */
    public readonly hasMockups = input<boolean>(false);
    public readonly isSending = input<boolean>(false);

    public readonly send = output<void>();
    public readonly approve = output<void>();

    protected readonly AgentFeedbackAction = AgentFeedbackAction;

    protected readonly view = computed(() =>
        AgentFeedbackConverter.toBarView(this.run(), this.feedback(), this.hasMockups())
    );
    protected readonly total = computed(() => AgentFeedbackConverter.total(this.feedback()));

    protected onSend(): void {
        if (!this.isSending()) {
            this.send.emit();
        }
    }
}

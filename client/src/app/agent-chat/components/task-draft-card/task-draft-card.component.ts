import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { TaskDraftConverter } from '../../entity/task-draft.converter';

/** A task the agent drafted, shown in the conversation until the owner saves it. */
@Component({
    selector: 'app-task-draft-card',
    templateUrl: './task-draft-card.component.html',
    styleUrls: ['./task-draft-card.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class TaskDraftCardComponent {
    /** The draft message: Markdown whose first `# ` line is the title. */
    public readonly body = input.required<string>();

    public readonly edit = output<void>();

    protected readonly draft = computed(() => TaskDraftConverter.toDraft(this.body()));
}

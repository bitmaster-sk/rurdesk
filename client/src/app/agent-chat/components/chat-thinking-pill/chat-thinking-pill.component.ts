import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';

/** The pill that opens and closes the thinking of a finished agent message. */
@Component({
    selector: 'app-chat-thinking-pill',
    templateUrl: './chat-thinking-pill.component.html',
    styleUrls: ['./chat-thinking-pill.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class ChatThinkingPillComponent {
    public readonly isOpen = input(false);
    public readonly toggled = output<void>();
}

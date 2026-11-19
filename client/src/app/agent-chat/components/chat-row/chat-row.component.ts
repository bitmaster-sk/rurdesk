import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { User } from 'src/app/auth/model/user.model';

/**
 * A conversation row laid out like `app-message-view`; a continuation shows no author.
 * `[messageMeta]` goes after the time, `[messageLead]` between the author line and the content.
 */
@Component({
    selector: 'app-chat-row',
    templateUrl: './chat-row.component.html',
    styleUrls: ['./chat-row.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class ChatRowComponent {
    public readonly author = input<User | null>(null);
    public readonly createdAt = input<Date | null>(null);
    public readonly isContinuation = input(false);
}

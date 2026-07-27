import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { Message } from 'src/app/message/model/message.model';
import { User } from 'src/app/auth/model/user.model';

@Component({
    selector: 'app-anchor-reply',
    templateUrl: './anchor-reply.component.html',
    styleUrls: ['./anchor-reply.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AnchorReplyComponent {
    public readonly reply = input.required<Message>();
    public readonly candidates = input<Map<number, User> | User[] | null>(null);
}

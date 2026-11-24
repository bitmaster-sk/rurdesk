import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { User } from 'src/app/auth/model/user.model';
import { Message } from 'src/app/message/model/message.model';

/** An anchored reply in the feed: who wrote it, what it is about, and a way into its thread. */
@Component({
    selector: 'app-anchor-reply-card',
    templateUrl: './anchor-reply-card.component.html',
    styleUrls: ['./anchor-reply-card.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AnchorReplyCardComponent {
    public readonly reply = input.required<Message>();
    /** Author of the message the thread is on; null when that message is not loaded. */
    public readonly parentAuthor = input<User | null>(null);
    public readonly candidates = input<Map<number, User> | User[] | null>(null);

    public readonly open = output<void>();

    protected readonly quote = computed(() => {
        const anchor = this.reply().anchor;
        return anchor?.quote ?? anchor?.anchorText ?? '';
    });

    /** A link or an attachment in the reply body does its own thing instead of opening the thread. */
    protected onClick(event: MouseEvent): void {
        if ((event.target as Element | null)?.closest('a, button')) {
            return;
        }
        this.open.emit();
    }
}

import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { User } from 'src/app/auth/model/user.model';
import { AnchorDraft, AnchorThread } from 'src/app/issue/entity/anchor-thread.entity';
import { Message } from 'src/app/message/model/message.model';
import { AttachmentScope } from 'src/app/shared/attachment/entity/attachment-scope.entity';
import { MENTION_TOKEN_RE } from 'src/app/shared/mention/mention';

const MAX_AVATARS = 3;

/**
 * A thread under the text it is about: a collapsed row, or the replies with a reply input.
 * Without a thread it is the composer of a new thread started from a selection.
 */
@Component({
    selector: 'app-anchor-thread',
    templateUrl: './anchor-thread.component.html',
    styleUrls: ['./anchor-thread.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AnchorThreadComponent {
    public readonly thread = input<AnchorThread | null>(null);
    public readonly draft = input<AnchorDraft | null>(null);
    public readonly isExpanded = input(false);
    public readonly unreadCount = input(0);
    public readonly currentUserId = input<number | null>(null);
    public readonly idMessageEdit = input<number | null>(null);
    public readonly candidates = input<Map<number, User> | User[] | null>(null);
    public readonly mentionCandidates = input<User[]>([]);
    public readonly attachmentScope = input<AttachmentScope | null>(null);

    public readonly toggleExpand = output<void>();
    public readonly reply = output<string>();
    public readonly cancelDraft = output<void>();
    public readonly editRequest = output<Message>();
    public readonly editSave = output<{ message: Message; text: string }>();
    public readonly editCancel = output<void>();

    protected readonly avatars = computed<User[]>(() => {
        const seen = new Map<number, User>();
        for (const reply of this.thread()?.replies ?? []) {
            seen.set(reply.creator.idUser, reply.creator);
        }
        return Array.from(seen.values()).slice(0, MAX_AVATARS);
    });

    protected readonly lastReply = computed<Message | null>(
        () => this.thread()?.replies.at(-1) ?? null
    );

    protected readonly lastReplyText = computed(() => {
        const text = this.lastReply()?.message ?? '';
        return text.replace(MENTION_TOKEN_RE, '@$1').replace(/\s+/g, ' ').trim();
    });

    protected readonly replyCountKey = computed(() =>
        this.thread()?.replies.length === 1
            ? 'MESSAGE.ANCHOR.REPLIES_ONE'
            : 'MESSAGE.ANCHOR.REPLIES_OTHER'
    );
}

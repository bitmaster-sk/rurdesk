import { Injectable, inject, signal } from '@angular/core';
import { EMPTY, Subject, catchError, debounceTime, filter, merge, switchMap } from 'rxjs';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { AgentConversationApi } from '../api/agent-conversation.api.service';
import { AgentConversationAuthor } from '../constants/agent-conversation-author.enum';

/** Wait for a burst of agent replies to settle before asking the server again. */
const REPLY_REFRESH_DEBOUNCE_MS = 300;

/**
 * Number of conversations with an agent reply the owner has not read yet. A reply or a read
 * refetches it from the server; nothing is counted locally.
 */
@Injectable({ providedIn: 'root' })
export class AgentConversationUnreadStore {
    private readonly api = inject(AgentConversationApi);
    private readonly sNotice = inject(NoticeService);

    private readonly _count = signal(0);
    public readonly count = this._count.asReadonly();

    private readonly refreshSubject = new Subject<void>();

    private readonly changedSubject = new Subject<void>();
    /** This tab created or read a conversation; no notice reports that back to it. */
    public readonly changed$ = this.changedSubject.asObservable();

    public constructor() {
        const agentReply$ = this.sNotice.agentConversation$.pipe(
            filter(
                notice =>
                    notice.action === NoticeAction.Create &&
                    notice.payload.author === AgentConversationAuthor.Agent
            ),
            debounceTime(REPLY_REFRESH_DEBOUNCE_MS)
        );
        // A failed request keeps the count and the stream alive.
        merge(agentReply$, this.refreshSubject)
            .pipe(switchMap(() => this.api.loadUnreadCount$().pipe(catchError(() => EMPTY))))
            .subscribe(count => this._count.set(count));
    }

    public load(): void {
        this.refreshSubject.next();
    }

    /** The server marked a conversation read: take the new count from it. */
    public markRead(): void {
        this.refreshSubject.next();
        this.notifyChanged();
    }

    public notifyChanged(): void {
        this.changedSubject.next();
    }
}

import { Injector, runInInjectionContext } from '@angular/core';
import { Subject, defer, of } from 'rxjs';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { NoticeSubject } from 'src/app/shared/notice/constant/notice-subject.enum';
import { Notice } from 'src/app/shared/notice/model/notice.model';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { AgentConversationApi } from '../api/agent-conversation.api.service';
import { AgentConversationMessage } from '../model/agent-conversation.model';
import { AgentConversationUnreadStore } from './agent-conversation-unread.store';
import { AgentConversationAuthor } from '../constants/agent-conversation-author.enum';

/** The server's unread state: the conversations with an agent reply not read yet. */
class FakeServer {
    public readonly unread = new Set<number>();
    public requests = 0;

    public reply(idConversation: number): void {
        this.unread.add(idConversation);
    }

    public read(idConversation: number): void {
        this.unread.delete(idConversation);
    }
}

function buildStore(server: FakeServer): {
    store: AgentConversationUnreadStore;
    notices: Subject<Notice<AgentConversationMessage>>;
} {
    const notices = new Subject<Notice<AgentConversationMessage>>();
    const api = {
        loadUnreadCount$: () =>
            defer(() => {
                server.requests++;
                return of(server.unread.size);
            })
    };
    const injector = Injector.create({
        providers: [
            { provide: AgentConversationApi, useValue: api },
            { provide: NoticeService, useValue: { agentConversation$: notices.asObservable() } }
        ]
    });
    const store = runInInjectionContext(injector, () => new AgentConversationUnreadStore());
    return { store, notices };
}

function notice(
    idConversation: number,
    author: AgentConversationMessage['author'],
    action = NoticeAction.Create
): Notice<AgentConversationMessage> {
    return {
        subject: NoticeSubject.AgentConversation,
        action,
        payload: { idConversation, author } as unknown as AgentConversationMessage
    };
}

describe('AgentConversationUnreadStore', () => {
    beforeEach(() => {
        vi.useFakeTimers();
    });

    afterEach(() => {
        vi.useRealTimers();
    });

    /** An agent reply: the server stores it unread, then the notice reaches the tab. */
    function agentReply(
        server: FakeServer,
        notices: Subject<Notice<AgentConversationMessage>>,
        idConversation: number
    ): void {
        server.reply(idConversation);
        notices.next(notice(idConversation, AgentConversationAuthor.Agent));
    }

    it('load() sets the count from the server', () => {
        const server = new FakeServer();
        [1, 2, 3].forEach(id => server.reply(id));
        const { store } = buildStore(server);

        store.load();

        expect(store.count()).toBe(3);
    });

    it('a new agent reply refreshes the count from the server', () => {
        const server = new FakeServer();
        server.reply(1);
        const { store, notices } = buildStore(server);
        store.load();

        agentReply(server, notices, 7);
        vi.advanceTimersByTime(300);

        expect(store.count()).toBe(2);
    });

    it('a burst of replies refreshes the count once', () => {
        const server = new FakeServer();
        const { store, notices } = buildStore(server);

        agentReply(server, notices, 7);
        agentReply(server, notices, 7);
        agentReply(server, notices, 8);
        vi.advanceTimersByTime(300);

        expect(store.count()).toBe(2);
        expect(server.requests).toBe(1);
    });

    it('ignores the owner own messages and non-create notices', () => {
        const server = new FakeServer();
        const { store, notices } = buildStore(server);

        notices.next(notice(7, AgentConversationAuthor.User));
        notices.next(notice(7, AgentConversationAuthor.System));
        notices.next(notice(7, AgentConversationAuthor.Agent, NoticeAction.Update));
        vi.advanceTimersByTime(300);

        expect(server.requests).toBe(0);
        expect(store.count()).toBe(0);
    });

    // The badge drifted: load() counted A, a second reply in A counted it again,
    // and opening A left 1 with nothing unread.
    it('already unread at load, then another reply, then open: 0', () => {
        const server = new FakeServer();
        server.reply(5);
        const { store, notices } = buildStore(server);
        store.load();
        expect(store.count()).toBe(1);

        agentReply(server, notices, 5);
        vi.advanceTimersByTime(300);
        expect(store.count()).toBe(1);

        server.read(5);
        store.markRead();

        expect(store.count()).toBe(0);
    });

    it('markRead() resyncs with the server instead of guessing', () => {
        const server = new FakeServer();
        [5, 6].forEach(id => server.reply(id));
        const { store } = buildStore(server);
        store.load();

        server.read(5);
        store.markRead();
        store.markRead();

        expect(store.count()).toBe(1);
    });

    // The top menu keeps its own list, so it must hear about changes this tab made.
    it('reports a read or a change made in this tab to whoever shows the list', () => {
        const server = new FakeServer();
        const { store } = buildStore(server);
        let changes = 0;
        store.changed$.subscribe(() => changes++);

        store.markRead();
        store.notifyChanged();

        expect(changes).toBe(2);
    });
});

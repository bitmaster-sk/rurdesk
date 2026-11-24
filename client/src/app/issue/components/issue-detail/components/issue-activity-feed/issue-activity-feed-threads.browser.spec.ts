import { Component, Pipe, PipeTransform, input, output, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { NEVER, Subject, of } from 'rxjs';
import { TranslateModule } from '@ngx-translate/core';
import {
    AvatarStub,
    MessageBodyStub,
    MessageEditorStub,
    TablerIconStub,
    UiTagStub
} from 'src/testing/stubs';
import { IssueActivityFeedComponent } from './issue-activity-feed.component';
import { AnchorReplyCardComponent } from '../anchor-reply-card/anchor-reply-card.component';
import { User } from 'src/app/auth/model/user.model';
import { MessageApi } from 'src/app/message/api/message.api.service';
import { Message, MessageAnchor } from 'src/app/message/model/message.model';
import { MessageKind } from 'src/app/message/constant/message-kind.enum';
import { MessageRecipientType } from 'src/app/message/constant/message-recipient-type.enum';
import { AnchorDraft, AnchorThread } from 'src/app/issue/entity/anchor-thread.entity';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { NoticeSubject } from 'src/app/shared/notice/constant/notice-subject.enum';
import { Notice } from 'src/app/shared/notice/model/notice.model';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { AgentThinkingApi } from 'src/app/agent/api/agent-thinking.api.service';
import { WikiApi } from 'src/app/wiki/api/wiki.api.service';
import { IssueHistoryApi } from 'src/app/issue/api/issue-history.api.service';
import { StateStore } from 'src/app/state/store/state.store';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { IssueTypeStore } from 'src/app/issue-type/store/issue-type.store';
import { TagStore } from 'src/app/tag/store/tag.store';

@Pipe({ name: 'date', standalone: false })
class StubDatePipe implements PipeTransform {
    public transform(value: unknown): string {
        return String(value);
    }
}

/** Renders one placeholder per thread so the feed can scroll to it. */
@Component({
    selector: 'app-activity-comment-item',
    template: `
        @for (thread of threads(); track thread.idThread) {
            <div class="thread-stub" [id]="'anchor-thread-' + thread.idThread"></div>
        }
    `,
    standalone: true
})
class ActivityCommentItemStub {
    public readonly message = input<Message | undefined>(undefined);
    public readonly isOwnMessage = input<boolean>(false);
    public readonly isEditing = input<boolean>(false);
    public readonly threads = input<AnchorThread[]>([]);
    public readonly draft = input<AnchorDraft | null>(null);
    public readonly expandedThreads = input<ReadonlySet<number>>(new Set());
    public readonly unreadByThread = input<ReadonlyMap<number, number>>(new Map());
    public readonly currentUserId = input<unknown>(undefined);
    public readonly idThreadMessageEdit = input<unknown>(undefined);
    public readonly agentRun = input<unknown>(undefined);
    public readonly isLatestPlanForStage = input<boolean>(false);
    public readonly candidates = input<unknown>(undefined);
    public readonly mentionCandidates = input<User[]>([]);
    public readonly editRequest = output<void>();
    public readonly editSave = output<string>();
    public readonly editCancel = output<void>();
    public readonly anchorRequest = output<AnchorDraft>();
    public readonly draftSend = output<string>();
    public readonly draftCancel = output<void>();
    public readonly threadToggle = output<number>();
    public readonly threadReply = output<{ thread: AnchorThread; text: string }>();
    public readonly threadEditRequest = output<Message>();
    public readonly threadEditSave = output<{ message: Message; text: string }>();
    public readonly threadEditCancel = output<void>();
    public readonly approve = output<void>();
    public readonly approveMockup = output<string>();
}

const ME: User = { idUser: 1, name: 'Me', email: '', colorAvatarBg: '' };
const AGENT: User = { idUser: 2, name: 'Agent', email: '', colorAvatarBg: '' };

function message(idMessage: number, minute: number, overrides: Partial<Message> = {}): Message {
    return {
        idMessage,
        message: `message ${idMessage}`,
        messageKind: MessageKind.Comment,
        createdAt: new Date(2026, 10, 23, 10, minute),
        isRead: true,
        creator: ME,
        idRecipient: 7,
        idMessageRecipientType: MessageRecipientType.issue,
        version: 1,
        anchor: null,
        ...overrides
    };
}

function notice(action: NoticeAction, payload: Message): Notice<Message> {
    return { subject: NoticeSubject.Message, action, payload };
}

function anchor(overrides: Partial<MessageAnchor> = {}): MessageAnchor {
    return {
        idParentMessage: 1,
        parentVersion: 1,
        anchorLineStart: 2,
        anchorLineEnd: 2,
        isOutdated: false,
        anchorText: '2. Backfill',
        quote: 'Backfill',
        ...overrides
    };
}

describe('IssueActivityFeedComponent anchored threads (browser)', () => {
    let messages: Message[];
    let messageNotices: Subject<Notice<Message>>;
    let api: {
        load$: ReturnType<typeof vi.fn>;
        insert$: ReturnType<typeof vi.fn>;
        update$: ReturnType<typeof vi.fn>;
        markRead$: ReturnType<typeof vi.fn>;
    };

    beforeEach(async () => {
        messages = [
            message(1, 0, { creator: AGENT, message: '# Plan\n2. Backfill' }),
            message(2, 1, { creator: AGENT, isRead: false, anchor: anchor() }),
            message(3, 2, { anchor: anchor({ idThreadMessage: 2 }) })
        ];
        messageNotices = new Subject();
        api = {
            load$: vi.fn(() => of(messages)),
            insert$: vi.fn((_id: number, _type: number, text: string) =>
                of(message(10, 30, { message: text }))
            ),
            update$: vi.fn((idMessage: number, text: string) =>
                of(message(idMessage, 0, { message: text, version: 2 }))
            ),
            markRead$: vi.fn(() => of(undefined))
        };

        await TestBed.configureTestingModule({
            declarations: [IssueActivityFeedComponent, AnchorReplyCardComponent, StubDatePipe],
            imports: [
                TranslateModule.forRoot(),
                MessageEditorStub,
                TablerIconStub,
                AvatarStub,
                MessageBodyStub,
                UiTagStub,
                ActivityCommentItemStub
            ],
            providers: [
                { provide: AgentThinkingApi, useValue: { load$: () => NEVER } },
                {
                    provide: IssueHistoryApi,
                    useValue: { load$: () => of({ history: [], users: [] }) }
                },
                { provide: StateStore, useValue: { statesMapByProject$: () => NEVER } },
                { provide: SeverityStore, useValue: { severitiesMap$: NEVER } },
                { provide: IssueTypeStore, useValue: { issueTypesMap$: NEVER } },
                { provide: TagStore, useValue: { tagsMap$: NEVER } },
                { provide: WikiApi, useValue: { loadIssueProposals$: () => of([]) } },
                { provide: MessageApi, useValue: api },
                {
                    provide: TrackerService,
                    useValue: { loadTracks$: () => of([]), tracksChanged$: NEVER }
                },
                {
                    provide: NoticeService,
                    useValue: {
                        Message: messageNotices.asObservable(),
                        agentThinking$: NEVER,
                        wikiProposal$: NEVER,
                        issue$: NEVER
                    }
                },
                {
                    provide: ProjectMemberStore,
                    useValue: { load: () => {}, usersMap$: of(new Map<number, User>()) }
                },
                { provide: AuthStore, useValue: { user: signal(ME), getUser: () => ME } }
            ]
        }).compileComponents();
    });

    function render() {
        const fixture = TestBed.createComponent(IssueActivityFeedComponent);
        fixture.componentRef.setInput('idIssue', 7);
        fixture.componentRef.setInput('idIssuePublic', 31);
        fixture.componentRef.setInput('idProject', 10);
        fixture.detectChanges();
        const el = fixture.nativeElement as HTMLElement;
        const parentItem = (): ActivityCommentItemStub =>
            fixture.debugElement.query(By.directive(ActivityCommentItemStub)).componentInstance;
        const cards = (): HTMLElement[] =>
            Array.from(el.querySelectorAll<HTMLElement>('[data-testid="anchor-reply-card"]'));
        return { fixture, el, parentItem, cards };
    }

    it('marks the issue read once its conversation is loaded', () => {
        render();
        expect(api.markRead$).toHaveBeenCalledWith(7, MessageRecipientType.issue);
    });

    it('shows each anchored reply as a card in time order with its quote', () => {
        const page = render();

        expect(page.el.querySelectorAll('app-activity-comment-item')).toHaveLength(1);
        expect(page.cards()).toHaveLength(2);
        expect(page.cards()[0].textContent).toContain('Backfill');
        expect(page.cards()[0].textContent).toContain('MESSAGE.ANCHOR.IN_THREAD');
    });

    it('keeps the cards under the comment filter chip', () => {
        const page = render();
        page.fixture.componentInstance.setFilter('comment');
        page.fixture.detectChanges();

        expect(page.cards()).toHaveLength(2);
    });

    it('groups replies into one thread on the parent and counts the unread ones', () => {
        const page = render();
        const thread = page.parentItem().threads()[0];

        expect(page.parentItem().threads()).toHaveLength(1);
        expect(thread.idThread).toBe(2);
        expect(thread.replies.map(reply => reply.idMessage)).toEqual([2, 3]);
        expect(page.parentItem().unreadByThread().get(2)).toBe(1);
    });

    it('opens the thread of a clicked card, scrolls to it and offers a way back', async () => {
        const scrolled: string[] = [];
        const scrollSpy = vi
            .spyOn(Element.prototype, 'scrollIntoView')
            .mockImplementation(function (this: Element) {
                scrolled.push(this.id);
            });
        const page = render();

        page.cards()[1].click();
        page.fixture.detectChanges();
        await new Promise(resolve => setTimeout(resolve));

        expect(page.parentItem().expandedThreads().has(2)).toBe(true);
        expect(page.parentItem().unreadByThread().has(2)).toBe(false);
        expect(scrolled).toContain('anchor-thread-2');

        const back = page.el.querySelector<HTMLElement>('[data-testid="anchor-back"]')!;
        back.click();
        page.fixture.detectChanges();

        expect(scrolled.at(-1)).toBe('anchor-card-3');
        expect(page.el.querySelector('[data-testid="anchor-back"]')).toBeNull();
        scrollSpy.mockRestore();
    });

    it('clears a filter that hides the parent of an opened thread', async () => {
        messages[2] = message(3, 2, {
            message: 'see [log](attachment:0b6f2c1e-3c8a-4f7e-9d2a-1a2b3c4d5e6f)',
            anchor: anchor({ idThreadMessage: 2 })
        });
        const scrollSpy = vi
            .spyOn(Element.prototype, 'scrollIntoView')
            .mockImplementation(() => {});
        const page = render();
        page.fixture.componentInstance.setFilter('attachment');
        page.fixture.detectChanges();
        expect(page.el.querySelector('app-activity-comment-item')).toBeNull();

        page.cards()[0].click();
        page.fixture.detectChanges();
        await new Promise(resolve => setTimeout(resolve));

        expect(page.fixture.componentInstance.activeFilters().size).toBe(0);
        expect(page.parentItem().expandedThreads().has(2)).toBe(true);
        expect(scrollSpy).toHaveBeenCalled();
        scrollSpy.mockRestore();
    });

    it('counts no reply as new that arrived while its thread was open', () => {
        const page = render();
        page.parentItem().threadToggle.emit(2);
        messageNotices.next(
            notice(
                NoticeAction.Create,
                message(4, 5, { creator: AGENT, anchor: anchor({ idThreadMessage: 2 }) })
            )
        );
        page.parentItem().threadToggle.emit(2);
        page.fixture.detectChanges();

        expect(page.parentItem().expandedThreads().has(2)).toBe(false);
        expect(page.parentItem().unreadByThread().has(2)).toBe(false);
    });

    it('tags the card of a reply to an older version', () => {
        messages[1] = message(2, 1, { anchor: anchor({ isOutdated: true }) });
        const page = render();

        expect(
            page.cards()[0].querySelector('[data-testid="anchor-reply-card-outdated"]')
        ).not.toBeNull();
        expect(
            page.cards()[1].querySelector('[data-testid="anchor-reply-card-outdated"]')
        ).toBeNull();
    });

    it('adds a reply from the websocket to the feed and to its thread as unread', () => {
        const page = render();
        messageNotices.next(
            notice(
                NoticeAction.Create,
                message(4, 5, { creator: AGENT, anchor: anchor({ idThreadMessage: 2 }) })
            )
        );
        page.fixture.detectChanges();

        expect(page.cards()).toHaveLength(3);
        expect(page.parentItem().threads()[0].replies).toHaveLength(3);
        expect(page.parentItem().unreadByThread().get(2)).toBe(2);
    });

    it('posts a new thread from a selection with its lines and quote, then opens it', () => {
        api.insert$.mockReturnValueOnce(
            of(
                message(11, 40, {
                    anchor: anchor({ anchorLineStart: 1, anchorLineEnd: 1, quote: 'Plan' })
                })
            )
        );
        const page = render();
        page.parentItem().anchorRequest.emit({
            idParentMessage: 1,
            lineStart: 1,
            lineEnd: 1,
            quote: 'Plan'
        });
        page.fixture.detectChanges();
        expect(page.parentItem().draft()?.quote).toBe('Plan');

        page.parentItem().draftSend.emit('why?');
        page.fixture.detectChanges();

        expect(api.insert$).toHaveBeenCalledWith(7, MessageRecipientType.issue, 'why?', {
            idParentMessage: 1,
            anchorLineStart: 1,
            anchorLineEnd: 1,
            anchorQuote: 'Plan'
        });
        expect(page.parentItem().draft()).toBeNull();
        expect(page.parentItem().expandedThreads().has(11)).toBe(true);
    });

    it('cancels a draft without a request', () => {
        const page = render();
        page.parentItem().anchorRequest.emit({
            idParentMessage: 1,
            lineStart: 1,
            lineEnd: 1,
            quote: 'Plan'
        });
        page.parentItem().draftCancel.emit();
        page.fixture.detectChanges();

        expect(page.parentItem().draft()).toBeNull();
        expect(api.insert$).not.toHaveBeenCalled();
    });

    it('posts a reply typed in a thread to that thread', () => {
        const page = render();
        const thread = page.parentItem().threads()[0];
        page.parentItem().threadReply.emit({ thread, text: 'agreed' });

        expect(api.insert$).toHaveBeenCalledWith(7, MessageRecipientType.issue, 'agreed', {
            idThreadMessage: 2
        });
    });

    it('reloads the conversation when a message with threads is edited over the websocket', () => {
        render();
        expect(api.load$).toHaveBeenCalledTimes(1);

        messageNotices.next(
            notice(
                NoticeAction.Update,
                message(1, 0, {
                    creator: AGENT,
                    message: 'top\n# Plan\n2. Backfill',
                    version: 2
                })
            )
        );

        expect(api.load$).toHaveBeenCalledTimes(2);
    });

    it('keeps the rendered parent, and what is typed in it, across that reload', () => {
        const page = render();
        const before = page.parentItem();

        messages = [
            message(1, 0, { creator: AGENT, message: 'top\n# Plan\n2. Backfill', version: 2 }),
            ...messages.slice(1)
        ];
        messageNotices.next(notice(NoticeAction.Update, messages[0]));
        page.fixture.detectChanges();

        expect(api.load$).toHaveBeenCalledTimes(2);
        expect(page.parentItem()).toBe(before);
        expect(page.parentItem().message()?.message).toBe('top\n# Plan\n2. Backfill');
    });

    it('keeps an unopened thread new across that reload, though the server counts it read', () => {
        const page = render();
        messages = [
            message(1, 0, { creator: AGENT, message: 'top\n# Plan\n2. Backfill', version: 2 }),
            ...messages.slice(1).map(reply => ({ ...reply, isRead: true }))
        ];
        messageNotices.next(notice(NoticeAction.Update, messages[0]));
        page.fixture.detectChanges();

        expect(api.load$).toHaveBeenCalledTimes(2);
        expect(page.parentItem().unreadByThread().get(2)).toBe(1);
    });

    it('tells the server a reply was read when it arrives in an open thread', () => {
        const page = render();
        page.parentItem().threadToggle.emit(2);
        api.markRead$.mockClear();

        messageNotices.next(
            notice(
                NoticeAction.Create,
                message(4, 5, { creator: AGENT, anchor: anchor({ idThreadMessage: 2 }) })
            )
        );

        expect(api.markRead$).toHaveBeenCalledWith(7, MessageRecipientType.issue);
    });

    it('leaves the server read state alone for a reply to a closed thread', () => {
        render();
        api.markRead$.mockClear();

        messageNotices.next(
            notice(
                NoticeAction.Create,
                message(4, 5, { creator: AGENT, anchor: anchor({ idThreadMessage: 2 }) })
            )
        );

        expect(api.markRead$).not.toHaveBeenCalled();
    });

    it('patches a message without threads in place when it is edited', () => {
        messages = [message(1, 0)];
        render();
        messageNotices.next(
            notice(NoticeAction.Update, message(1, 0, { message: 'changed', version: 2 }))
        );

        expect(api.load$).toHaveBeenCalledTimes(1);
    });
});

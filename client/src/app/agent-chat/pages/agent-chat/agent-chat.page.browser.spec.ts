import { Component, input, output } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import {
    ActivatedRoute,
    convertToParamMap,
    ParamMap,
    provideRouter,
    Router,
    RouterModule
} from '@angular/router';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { page } from '@vitest/browser/context';
import { BehaviorSubject, of, Subject, throwError } from 'rxjs';
import { AgentThinkingKind } from 'src/app/agent/constants/agent-thinking-kind.enum';
import {
    AgentThinkingEvent,
    AgentThinkingNotice,
    AgentThinkingRes
} from 'src/app/agent/model/agent-thinking.model';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { ProjectApi } from 'src/app/project/api/project.api.service';
import { Project } from 'src/app/project/model/project.model';
import { UiToastService } from 'src/app/ui/service/ui-toast.service';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { NoticeSubject } from 'src/app/shared/notice/constant/notice-subject.enum';
import { Notice } from 'src/app/shared/notice/model/notice.model';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { Fixtures } from 'src/testing/fixtures';
import {
    AvatarStub,
    MessageEditorStub,
    TablerIconStub,
    UiButtonStub,
    UiChoiceStub,
    UiConfirmStub,
    UiTooltipStub
} from 'src/testing/stubs';
import { AgentConversationApi } from '../../api/agent-conversation.api.service';
import { MessageViewComponent } from 'src/app/message/components/message-view/message-view.component';
import { ChatRowComponent } from '../../components/chat-row/chat-row.component';
import { ChatThinkingComponent } from '../../components/chat-thinking/chat-thinking.component';
import { ChatThinkingPillComponent } from '../../components/chat-thinking-pill/chat-thinking-pill.component';
import { LiveThinking } from '../../entity/live-thinking.entity';
import { AgentThinkingLinesComponent } from 'src/app/shared/components/agent-thinking-lines/agent-thinking-lines.component';
import { ConversationListComponent } from '../../components/conversation-list/conversation-list.component';
import {
    AgentConversation,
    AgentConversationDetail,
    AgentConversationListItem,
    AgentConversationMessage,
    AgentConversationSettings,
    AgentConversationTurnNotice,
    AgentStatus
} from '../../model/agent-conversation.model';
import { AgentConversationUnreadStore } from '../../store/agent-conversation-unread.store';
import { AgentChatPage } from './agent-chat.page';
import { UiButtonComponent } from 'src/app/ui/components/button/button.component';
import { AgentConversationAuthor } from '../../constants/agent-conversation-author.enum';
import { AgentConversationMessageKind } from '../../constants/agent-conversation-message-kind.enum';
import { AgentStatusState } from '../../constants/agent-status-state.enum';

@Component({ selector: 'app-message-view', template: '<ng-content />', standalone: true })
class MessageViewStub {
    public readonly message = input<{ message: string } | undefined>(undefined);
    public readonly isContinuation = input<boolean>(false);
    public readonly isOwnMessage = input<boolean>(false);
    public readonly candidates = input<unknown>(undefined);
}

@Component({ selector: 'app-conversation-header', template: '', standalone: true })
class ConversationHeaderStub {
    public readonly detail = input<AgentConversationDetail | null>(null);
    public readonly projects = input<unknown[]>([]);
    public readonly projectError = input<string | null>(null);
    public readonly attachProject = output<number>();
    public readonly detachProject = output<number>();
    public readonly pinChange = output<boolean>();
    public readonly settingsChange = output<AgentConversationSettings>();
    public readonly deleteConversation = output<void>();
    public readonly back = output<void>();
}

@Component({ selector: 'app-task-draft-card', template: '', standalone: true })
class TaskDraftCardStub {
    public readonly body = input<string>('');
    public readonly edit = output<void>();
}

@Component({ selector: 'app-task-draft-dialog', template: '', standalone: true })
class TaskDraftDialogStub {
    public readonly body = input<string>('');
    public readonly idConversation = input<number>(0);
    public readonly idProject = input<number | null>(null);
    public readonly projects = input<unknown[]>([]);
    public readonly cancelled = output<void>();
}

@Component({ selector: 'app-new-conversation', template: '', standalone: true })
class NewConversationStub {
    public readonly projects = input<unknown[]>([]);
    public readonly idProject = input<number | null>(null);
    public readonly created = output<AgentConversation>();
    public readonly attachRefused = output<Project[]>();
    public readonly attachFailed = output<Project[]>();
    public readonly back = output<void>();
}

@Component({ selector: 'app-chat-thinking', template: '', standalone: true })
class ChatThinkingStub {
    public readonly idConversation = input<number>(0);
    public readonly idRun = input<number | null>(null);
    public readonly idTask = input<number | null>(null);
    public readonly isLive = input<boolean>(false);
    public readonly events = input<AgentThinkingEvent[]>([]);
    public readonly status = input<string>('ready');
    public readonly thinkingChange = output<LiveThinking>();
}

const owner = Fixtures.user({ idUser: 1, name: 'Ada' });
const agent = Fixtures.agent({ idUser: 9, name: 'Agent 003' });

function conversation(overrides: Partial<AgentConversation> = {}): AgentConversation {
    return {
        idConversation: 4,
        idUser: 1,
        idUserAgent: 9,
        idProject: 7,
        idsProjectContext: [],
        idRun: 30,
        title: 'Why is the worktree kept?',
        headCommit: null,
        isLocked: true,
        taskDraftRequest: null,
        readAt: '2026-11-16T10:00:00Z',
        pinnedAt: null,
        model: null,
        lastAgentMessageAt: '2026-11-16T09:00:00Z',
        createdAt: '2026-11-16T09:00:00Z',
        updatedAt: '2026-11-16T10:00:00Z',
        ...overrides
    };
}

function listItem(idConversation: number, title: string): AgentConversationListItem {
    return {
        ...conversation({ idConversation, title }),
        isUnread: false,
        tokensUsed: 1200,
        costUsd: null,
        costEstimated: false
    };
}

function thought(text: string): AgentThinkingEvent {
    return { kind: AgentThinkingKind.Thinking, text, at: 1 };
}

function storedThinking(events: AgentThinkingEvent[]): AgentThinkingRes {
    return { idRun: 30, idTask: 3, stage: 'chat', events, isComplete: true };
}

function message(overrides: Partial<AgentConversationMessage>): AgentConversationMessage {
    return {
        idMessage: 10,
        idConversation: 4,
        author: AgentConversationAuthor.User,
        kind: AgentConversationMessageKind.Text,
        body: 'Where is the worktree removed?',
        idUser: 1,
        idTask: null,
        hasThinking: false,
        createdAt: '2026-11-16T09:00:00Z',
        ...overrides
    };
}

function detail(
    overrides: Partial<AgentConversation> = {},
    status: Partial<AgentStatus> = {}
): AgentConversationDetail {
    return {
        conversation: conversation(overrides),
        projects: [{ idProject: 7, name: 'rurdesk', isMain: true }],
        agent,
        agentStatus: {
            state: AgentStatusState.Idle,
            idIssuePublic: null,
            idProject: null,
            stage: null,
            queuePosition: null,
            idTask: null,
            ...status
        },
        agentModels: { models: [], modelDefault: null },
        stats: null
    };
}

function conversationNotice(payload: AgentConversationMessage): Notice<AgentConversationMessage> {
    return { subject: NoticeSubject.AgentConversation, action: NoticeAction.Create, payload };
}

// The component specs load no global stylesheet: give the spacing tokens the list uses.
function withSpacingTokens(): void {
    const style = document.createElement('style');
    style.textContent =
        ':root { --ui-space-xs: 0.125rem; --ui-space: 0.5rem; --ui-space-s: 0.25rem; --ui-space-l: 1rem; --ui-space-xl: 1.5rem; }';
    document.head.appendChild(style);
    onTestFinished(() => style.remove());
}

function expectInsideBorder(host: HTMLElement): void {
    withSpacingTokens();
    const rows = host.querySelectorAll<HTMLElement>('.conversation-list-row');
    expect(rows.length).toBeGreaterThan(0);
    rows.forEach(row => {
        const item = row.querySelector<HTMLElement>('[data-testid="chat-list-item"]')!;
        const itemRect = item.getBoundingClientRect();
        const border = parseFloat(getComputedStyle(item).borderTopWidth);
        const buttons = row.querySelectorAll<HTMLElement>(
            '.conversation-list-item-actions ui-button'
        );
        expect(buttons.length).toBe(2);
        buttons.forEach(button => {
            const rect = button.getBoundingClientRect();
            expect(rect.left).toBeGreaterThanOrEqual(itemRect.left + border);
            expect(rect.right).toBeLessThanOrEqual(itemRect.right - border);
            expect(rect.top).toBeGreaterThanOrEqual(itemRect.top + border);
            expect(rect.bottom).toBeLessThanOrEqual(itemRect.bottom - border);
        });
    });
}

describe('AgentChatPage (browser)', () => {
    let fixture: ComponentFixture<AgentChatPage>;
    let paramMap: BehaviorSubject<ParamMap>;
    let queryParamMap: BehaviorSubject<ParamMap>;
    let conversationNotices: Subject<Notice<AgentConversationMessage>>;
    let turnNotices: Subject<Notice<AgentConversationTurnNotice>>;
    let thinkingNotices: Subject<{ payload: AgentThinkingNotice }>;
    let api: Record<string, ReturnType<typeof vi.fn>>;
    let toast: { show: ReturnType<typeof vi.fn> };
    let unreadStore: {
        markRead: ReturnType<typeof vi.fn>;
        notifyChanged: ReturnType<typeof vi.fn>;
    };

    beforeEach(async () => {
        await page.viewport(1280, 800);
        toast = { show: vi.fn() };
        paramMap = new BehaviorSubject(convertToParamMap({}));
        queryParamMap = new BehaviorSubject(convertToParamMap({}));
        conversationNotices = new Subject();
        turnNotices = new Subject();
        thinkingNotices = new Subject();
        api = {
            load$: vi.fn((idProject?: number | null) =>
                of(
                    idProject == null
                        ? [listItem(4, 'In rurdesk'), listItem(5, 'Elsewhere')]
                        : [listItem(4, 'In rurdesk')]
                )
            ),
            loadOne$: vi.fn(() => of(detail())),
            loadMessages$: vi.fn(() => of([message({})])),
            insertMessage$: vi.fn(),
            markRead$: vi.fn(() => of(undefined)),
            loadAgents$: vi.fn(() => of([agent])),
            insert$: vi.fn(),
            delete$: vi.fn(() => of(undefined)),
            insertProject$: vi.fn(() => of(undefined)),
            deleteProject$: vi.fn(() => of(undefined)),
            retry$: vi.fn(() => of(undefined)),
            setPinned$: vi.fn(() => of(undefined)),
            updateSettings$: vi.fn(() => of(undefined)),
            insertTaskDraft$: vi.fn(() => of(undefined)),
            loadThinking$: vi.fn(() => of(storedThinking([thought('reading gc.go')])))
        };
        unreadStore = { markRead: vi.fn(), notifyChanged: vi.fn() };
        await TestBed.configureTestingModule({
            declarations: [
                AgentChatPage,
                ConversationListComponent,
                ChatRowComponent,
                ChatThinkingPillComponent
            ],
            imports: [
                TranslateModule.forRoot(),
                ReactiveFormsModule,
                RouterModule,
                MessageViewStub,
                MessageEditorStub,
                ConversationHeaderStub,
                NewConversationStub,
                ChatThinkingStub,
                TaskDraftCardStub,
                TaskDraftDialogStub,
                AvatarStub,
                TablerIconStub,
                UiButtonStub,
                UiChoiceStub,
                UiConfirmStub,
                UiTooltipStub
            ],
            providers: [
                provideRouter([]),
                { provide: ActivatedRoute, useValue: { paramMap, queryParamMap } },
                { provide: AgentConversationApi, useValue: api },
                { provide: ProjectApi, useValue: { load$: () => of([Fixtures.project()]) } },
                { provide: UiToastService, useValue: toast },
                { provide: AuthStore, useValue: { getUser: () => owner } },
                { provide: AgentConversationUnreadStore, useValue: unreadStore },
                {
                    provide: NoticeService,
                    useValue: {
                        agentConversation$: conversationNotices,
                        agentConversationTurn$: turnNotices,
                        agentThinking$: thinkingNotices
                    }
                }
            ]
        }).compileComponents();
        const translate = TestBed.inject(TranslateService);
        translate.setTranslation('en', {
            AGENT_CHAT: {
                NEW_FORM: {
                    PROJECTS_REFUSED: 'Not added: {{names}}.',
                    PROJECTS_FAILED: 'Failed: {{names}}.'
                }
            },
            error: { agent_not_in_project: 'Members only.' }
        });
        translate.use('en');
    });

    function render(params: Record<string, string> = {}, query: Record<string, string> = {}) {
        paramMap.next(convertToParamMap(params));
        queryParamMap.next(convertToParamMap(query));
        fixture = TestBed.createComponent(AgentChatPage);
        fixture.detectChanges();
        return fixture.nativeElement as HTMLElement;
    }

    function titles(host: HTMLElement): string[] {
        return Array.from(host.querySelectorAll('[data-testid="chat-list-item-title"]')).map(item =>
            item.textContent.trim()
        );
    }

    function bodies(): string[] {
        return fixture.debugElement
            .queryAll(debug => debug.name === 'app-message-view')
            .map(debug => debug.injector.get(MessageViewStub).message()!.message);
    }

    function send(text: string): void {
        fixture.debugElement
            .query(debug => debug.name === 'app-message-editor')
            .injector.get(MessageEditorStub)
            .messageChange.emit(text);
        fixture.detectChanges();
    }

    function choice(): UiChoiceStub {
        return fixture.debugElement
            .query(debug => debug.name === 'ui-choice')
            .injector.get(UiChoiceStub);
    }

    it('lists this project by default and every conversation after switching to All', () => {
        const host = render({}, { idProject: '7' });

        expect(api['load$']).toHaveBeenLastCalledWith(7);
        expect(titles(host)).toEqual(['In rurdesk']);

        choice().pick('all');
        fixture.detectChanges();

        expect(api['load$']).toHaveBeenLastCalledWith(null);
        expect(titles(host)).toEqual(['In rurdesk', 'Elsewhere']);
    });

    it('lists every conversation without a toggle when no project is given', () => {
        const host = render();

        expect(api['load$']).toHaveBeenLastCalledWith(null);
        expect(titles(host)).toEqual(['In rurdesk', 'Elsewhere']);
        expect(host.querySelector('ui-choice')).toBeNull();
    });

    it('opens the new conversation flow with the project preselected', () => {
        render({}, { new: '1', idProject: '7' });

        const flow = fixture.debugElement.query(debug => debug.name === 'app-new-conversation');
        expect(flow.injector.get(NewConversationStub).idProject()).toBe(7);
    });

    it('names the projects the server refused in one toast after the conversation opens', () => {
        vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
        render({}, { new: '1' });
        const flow = fixture.debugElement
            .query(debug => debug.name === 'app-new-conversation')
            .injector.get(NewConversationStub);

        flow.created.emit(conversation({ idConversation: 9 }));
        flow.attachRefused.emit([
            Fixtures.project({ name: 'Alpha' }),
            Fixtures.project({ name: 'Beta' })
        ]);

        expect(toast.show).toHaveBeenCalledTimes(1);
        expect(toast.show).toHaveBeenCalledWith(
            expect.objectContaining({
                severity: 'error',
                detail: expect.stringContaining('Alpha, Beta')
            })
        );
    });

    it('names the projects that failed for another reason without the membership wording', () => {
        vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
        render({}, { new: '1' });
        const flow = fixture.debugElement
            .query(debug => debug.name === 'app-new-conversation')
            .injector.get(NewConversationStub);

        flow.attachFailed.emit([Fixtures.project({ name: 'Gamma' })]);

        expect(toast.show).toHaveBeenCalledTimes(1);
        const detail = (toast.show.mock.calls[0][0] as { detail: string }).detail;
        expect(detail).toContain('Failed: Gamma.');
        expect(detail).not.toContain('Members only.');
    });

    it('shows a sent message at once and keeps one copy when the server confirms it', () => {
        const saved$ = new Subject<AgentConversationMessage>();
        api['insertMessage$'].mockReturnValue(saved$);
        const host = render({ idConversation: '4' });

        send('Does Continue keep it too?');

        expect(bodies()).toEqual(['Where is the worktree removed?', 'Does Continue keep it too?']);
        expect(host.querySelector('[data-testid="chat-message-pending"]')).not.toBeNull();

        const stored = message({ idMessage: 11, body: 'Does Continue keep it too?' });
        conversationNotices.next(conversationNotice(stored));
        saved$.next(stored);
        saved$.complete();
        fixture.detectChanges();

        expect(api['insertMessage$']).toHaveBeenCalledWith(4, 'Does Continue keep it too?');
        expect(bodies()).toEqual(['Where is the worktree removed?', 'Does Continue keep it too?']);
        expect(host.querySelector('[data-testid="chat-message-pending"]')).toBeNull();
    });

    it('shows why the message was refused above the composer', () => {
        api['insertMessage$'].mockReturnValue(
            throwError(() => ({
                status: 403,
                error: {
                    code: 'AGENT_CONVERSATION_NO_ACCESS',
                    translateKey: 'error.agent_conversation_no_access'
                }
            }))
        );
        const host = render({ idConversation: '4' });

        send('Hello?');

        expect(host.querySelector('[data-testid="agent-chat-send-error"]')!.textContent).toContain(
            'error.agent_conversation_no_access'
        );
        expect(bodies()).toEqual(['Where is the worktree removed?']);
    });

    it('replaces the composer with a hint while only context projects remain', () => {
        api['loadOne$'].mockReturnValue(
            of({
                ...detail({ idProject: null, idsProjectContext: [8], isLocked: false }),
                projects: [{ idProject: 8, name: 'gateway', isMain: false }]
            })
        );
        const host = render({ idConversation: '4' });

        expect(
            host.querySelector('[data-testid="agent-chat-main-required"]')!.textContent
        ).toContain('AGENT_CHAT.MAIN_REQUIRED');
        expect(host.querySelector('app-message-editor')).toBeNull();
    });

    it('keeps the composer when a main project is set', () => {
        const host = render({ idConversation: '4' });

        expect(host.querySelector('[data-testid="agent-chat-main-required"]')).toBeNull();
        expect(host.querySelector('app-message-editor')).not.toBeNull();
    });

    it('keeps the composer for a chat without any project', () => {
        api['loadOne$'].mockReturnValue(
            of({ ...detail({ idProject: null, isLocked: false }), projects: [] })
        );
        const host = render({ idConversation: '4' });

        expect(host.querySelector('[data-testid="agent-chat-main-required"]')).toBeNull();
        expect(host.querySelector('app-message-editor')).not.toBeNull();
    });

    it('marks an unread conversation read when it opens', () => {
        api['loadOne$'].mockReturnValue(
            of(detail({ readAt: null, lastAgentMessageAt: '2026-11-16T10:00:00Z' }))
        );
        render({ idConversation: '4' });

        expect(api['markRead$']).toHaveBeenCalledWith(4);
        expect(unreadStore.markRead).toHaveBeenCalled();
    });

    it('leaves a read conversation alone when it opens', () => {
        render({ idConversation: '4' });

        expect(api['markRead$']).not.toHaveBeenCalled();
        expect(unreadStore.markRead).not.toHaveBeenCalled();
    });

    it('shows a reply to the open conversation and keeps it read', () => {
        render({ idConversation: '4' });

        conversationNotices.next(
            conversationNotice(
                message({
                    idMessage: 12,
                    author: AgentConversationAuthor.Agent,
                    idUser: 9,
                    idTask: 3,
                    body: 'In gc.go'
                })
            )
        );
        fixture.detectChanges();

        expect(bodies()).toEqual(['Where is the worktree removed?', 'In gc.go']);
        expect(api['markRead$']).toHaveBeenCalledWith(4);
        expect(unreadStore.markRead).toHaveBeenCalled();
    });

    it('ignores messages of other conversations', () => {
        render({ idConversation: '4' });

        conversationNotices.next(
            conversationNotice(
                message({ idMessage: 12, idConversation: 5, author: AgentConversationAuthor.Agent })
            )
        );
        fixture.detectChanges();

        expect(bodies()).toEqual(['Where is the worktree removed?']);
        expect(api['markRead$']).not.toHaveBeenCalled();
    });

    it('shows a failed turn as a muted line', () => {
        api['loadMessages$'].mockReturnValue(
            of([
                message({}),
                message({
                    idMessage: 13,
                    author: AgentConversationAuthor.System,
                    kind: AgentConversationMessageKind.Error,
                    body: 'x'
                })
            ])
        );
        const host = render({ idConversation: '4' });

        expect(host.querySelectorAll('[data-testid="chat-system-line"]')).toHaveLength(1);
        expect(bodies()).toEqual(['Where is the worktree removed?']);
    });

    function liveBlock(): ChatThinkingStub | null {
        const debug = fixture.debugElement.query(item => item.name === 'app-chat-thinking');
        return debug ? debug.injector.get(ChatThinkingStub) : null;
    }

    function editor(): MessageEditorStub {
        return fixture.debugElement
            .query(debug => debug.name === 'app-message-editor')
            .injector.get(MessageEditorStub);
    }

    function retryButtons(host: HTMLElement): HTMLButtonElement[] {
        return Array.from(host.querySelectorAll('[data-testid="chat-retry"]'));
    }

    it('shows the agent answering once the queued turn starts, before any thinking', () => {
        api['loadOne$'].mockReturnValue(
            of(detail({}, { state: AgentStatusState.Queued, queuePosition: 1 }))
        );
        render({ idConversation: '4' });
        expect(liveBlock()).toBeNull();

        api['loadOne$'].mockReturnValue(
            of(detail({}, { state: AgentStatusState.Busy, stage: 'chat' }))
        );
        turnNotices.next({
            subject: NoticeSubject.AgentConversationTurn,
            action: NoticeAction.Update,
            payload: { idConversation: 4, idTask: 3 }
        });
        fixture.detectChanges();

        expect(liveBlock()!.isLive()).toBe(true);
        expect(liveBlock()!.idTask()).toBe(3);
    });

    it('ignores the turn of another conversation', () => {
        api['loadOne$'].mockReturnValue(
            of(detail({}, { state: AgentStatusState.Queued, queuePosition: 1 }))
        );
        render({ idConversation: '4' });
        api['loadOne$'].mockClear();

        turnNotices.next({
            subject: NoticeSubject.AgentConversationTurn,
            action: NoticeAction.Update,
            payload: { idConversation: 5, idTask: 3 }
        });
        fixture.detectChanges();

        expect(api['loadOne$']).not.toHaveBeenCalled();
        expect(liveBlock()).toBeNull();
    });

    // The first thinking of a turn names its task too, in case it arrives before the turn notice.
    it('starts live thinking from the notice that shows the queued turn began', () => {
        api['loadOne$'].mockReturnValue(
            of(detail({}, { state: AgentStatusState.Queued, queuePosition: 1 }))
        );
        render({ idConversation: '4' });
        expect(liveBlock()).toBeNull();

        api['loadOne$'].mockReturnValue(
            of(detail({}, { state: AgentStatusState.Busy, stage: 'chat' }))
        );
        thinkingNotices.next({
            payload: { idRun: 30, idTask: 3, stage: 'chat', seq: 1, events: [], idConversation: 4 }
        });
        fixture.detectChanges();

        expect(liveBlock()!.isLive()).toBe(true);
        expect(liveBlock()!.idRun()).toBe(30);
        expect(liveBlock()!.idTask()).toBe(3);
    });

    it('rejoins the running turn after a reload', () => {
        api['loadOne$'].mockReturnValue(
            of(detail({}, { state: AgentStatusState.Busy, stage: 'chat', idTask: 9 }))
        );
        render({ idConversation: '4' });

        expect(liveBlock()!.idTask()).toBe(9);
    });

    it('offers Retry only on the newest failed turn and queues it again', () => {
        api['loadMessages$'].mockReturnValue(
            of([
                message({}),
                message({
                    idMessage: 13,
                    author: AgentConversationAuthor.System,
                    kind: AgentConversationMessageKind.Error,
                    body: 'dispatch_failed'
                }),
                message({
                    idMessage: 14,
                    author: AgentConversationAuthor.Agent,
                    idUser: 9,
                    body: 'Here it is'
                }),
                message({
                    idMessage: 15,
                    author: AgentConversationAuthor.System,
                    kind: AgentConversationMessageKind.Error,
                    body: 'stage_not_submitted'
                })
            ])
        );
        const host = render({ idConversation: '4' });
        const lines = host.querySelectorAll('[data-testid="chat-system-line"]');

        expect(retryButtons(host)).toHaveLength(1);
        expect(lines[1].contains(retryButtons(host)[0])).toBe(true);

        retryButtons(host)[0].click();
        fixture.detectChanges();

        expect(api['retry$']).toHaveBeenCalledWith(4);
        expect(api['loadOne$']).toHaveBeenCalledTimes(2);
    });

    it('offers no Retry once a newer message follows the failure', () => {
        api['loadMessages$'].mockReturnValue(
            of([
                message({
                    idMessage: 13,
                    author: AgentConversationAuthor.System,
                    kind: AgentConversationMessageKind.Error,
                    body: 'x'
                }),
                message({ idMessage: 14, body: 'try again please' })
            ])
        );
        const host = render({ idConversation: '4' });

        expect(retryButtons(host)).toHaveLength(0);
    });

    it('shows why a retry was refused above the composer', () => {
        api['loadMessages$'].mockReturnValue(
            of([
                message({
                    idMessage: 13,
                    author: AgentConversationAuthor.System,
                    kind: AgentConversationMessageKind.Error,
                    body: 'x'
                })
            ])
        );
        api['retry$'].mockReturnValue(
            throwError(() => ({
                status: 409,
                error: { translateKey: 'error.agent_conversation_not_retryable' }
            }))
        );
        const host = render({ idConversation: '4' });

        retryButtons(host)[0].click();
        fixture.detectChanges();

        expect(host.querySelector('[data-testid="agent-chat-send-error"]')!.textContent).toContain(
            'error.agent_conversation_not_retryable'
        );
    });

    it('keeps Retry from being sent twice while the first one is on its way', () => {
        api['loadMessages$'].mockReturnValue(
            of([
                message({
                    idMessage: 13,
                    author: AgentConversationAuthor.System,
                    kind: AgentConversationMessageKind.Error,
                    body: 'x'
                })
            ])
        );
        const retried = new Subject<void>();
        api['retry$'].mockReturnValue(retried);
        const host = render({ idConversation: '4' });

        retryButtons(host)[0].click();
        fixture.detectChanges();

        expect(retryButtons(host)).toHaveLength(0);
        expect(api['retry$']).toHaveBeenCalledTimes(1);

        api['loadOne$'].mockReturnValue(
            of(detail({}, { state: AgentStatusState.Queued, queuePosition: 1 }))
        );
        retried.next();
        retried.complete();
        fixture.detectChanges();

        expect(retryButtons(host)).toHaveLength(0);
        expect(api['retry$']).toHaveBeenCalledTimes(1);
    });

    function draftDialog(): TaskDraftDialogStub | null {
        const debug = fixture.debugElement.query(item => item.name === 'app-task-draft-dialog');
        return debug ? debug.injector.get(TaskDraftDialogStub) : null;
    }

    const answered = [
        message({}),
        message({
            idMessage: 11,
            author: AgentConversationAuthor.Agent,
            kind: AgentConversationMessageKind.Text,
            body: 'The worktree is kept.'
        }),
        message({
            idMessage: 12,
            author: AgentConversationAuthor.User,
            kind: AgentConversationMessageKind.Text,
            body: 'Thanks'
        })
    ];

    function createTaskButtons(): { stub: UiButtonStub; host: HTMLElement }[] {
        return fixture.debugElement
            .queryAll(debug => debug.attributes['data-testid'] === 'chat-create-task')
            .map(debug => ({
                host: debug.nativeElement as HTMLElement,
                stub: debug.injector.get(UiButtonStub)
            }));
    }

    function renderAnswered(): void {
        api['loadMessages$'].mockReturnValue(of(answered));
        render({ idConversation: '4' });
    }

    it('offers Create task on agent answers only', () => {
        api['loadMessages$'].mockReturnValue(
            of([
                ...answered,
                message({
                    idMessage: 13,
                    author: AgentConversationAuthor.Agent,
                    kind: AgentConversationMessageKind.TaskDraft,
                    body: '# Draft'
                }),
                message({
                    idMessage: 14,
                    author: AgentConversationAuthor.System,
                    kind: AgentConversationMessageKind.Error,
                    body: 'stage_failed'
                })
            ])
        );
        render({ idConversation: '4' });

        expect(createTaskButtons()).toHaveLength(1);
        expect(createTaskButtons()[0].stub.disabled()).toBe(false);
    });

    it('shows the Create task tooltip on the left of the button', () => {
        renderAnswered();

        const tooltip = fixture.debugElement
            .query(debug => debug.attributes['data-testid'] === 'chat-create-task')
            .injector.get(UiTooltipStub);
        expect(tooltip.uiTooltipPosition()).toBe('left');
    });

    it('asks for a draft about the answer and waits while the draft turn is queued', () => {
        renderAnswered();

        api['loadOne$'].mockReturnValue(
            of(
                detail(
                    { taskDraftRequest: 'next_turn' },
                    { state: AgentStatusState.Queued, queuePosition: 1 }
                )
            )
        );
        createTaskButtons()[0].host.click();
        fixture.detectChanges();

        expect(api['insertTaskDraft$']).toHaveBeenCalledWith(4, 11);
        expect(createTaskButtons()[0].stub.disabled()).toBe(true);
    });

    it('asks for one draft however often Create task is pressed before it is accepted', () => {
        const requested = new Subject<void>();
        api['insertTaskDraft$'].mockReturnValue(requested);
        renderAnswered();

        createTaskButtons()[0].host.click();
        fixture.detectChanges();

        expect(createTaskButtons()[0].stub.disabled()).toBe(true);
        createTaskButtons()[0].host.click();
        expect(api['insertTaskDraft$']).toHaveBeenCalledTimes(1);
    });

    it.each([
        ['a turn is queued', detail({}, { state: AgentStatusState.Queued, queuePosition: 2 })],
        [
            'the agent is answering',
            detail({}, { state: AgentStatusState.Busy, stage: 'chat', idTask: 3 })
        ],
        ['a draft is already due', detail({ taskDraftRequest: 'after_turn' })],
        [
            'the conversation has context projects but no main one',
            {
                ...detail({ idsProjectContext: [8] }),
                projects: [{ idProject: 8, name: 'docs', isMain: false }]
            }
        ]
    ])('disables Create task while %s', (_, value) => {
        api['loadOne$'].mockReturnValue(of(value));
        renderAnswered();

        expect(createTaskButtons()[0].stub.disabled()).toBe(true);
    });

    it('shows a task draft as a card the owner edits into a task', () => {
        const draft = '# Keep the worktree\n\nUntil Restart.';
        api['loadMessages$'].mockReturnValue(
            of([
                message({}),
                message({
                    idMessage: 12,
                    author: AgentConversationAuthor.Agent,
                    kind: AgentConversationMessageKind.TaskDraft,
                    body: draft
                })
            ])
        );
        render({ idConversation: '4' });

        expect(bodies()).toEqual(['Where is the worktree removed?']);
        const card = fixture.debugElement
            .query(debug => debug.name === 'app-task-draft-card')
            .injector.get(TaskDraftCardStub);
        expect(card.body()).toBe(draft);
        expect(draftDialog()).toBeNull();

        card.edit.emit();
        fixture.detectChanges();

        expect(draftDialog()!.body()).toBe(draft);
        expect(draftDialog()!.idConversation()).toBe(4);
        expect(draftDialog()!.idProject()).toBe(7);

        draftDialog()!.cancelled.emit();
        fixture.detectChanges();

        expect(draftDialog()).toBeNull();
    });

    it('confirms a sent message when the response comes before the notice', () => {
        api['insertMessage$'].mockReturnValue(
            of(message({ idMessage: 11, body: 'Does Continue keep it too?' }))
        );
        const host = render({ idConversation: '4' });

        send('Does Continue keep it too?');
        conversationNotices.next(
            conversationNotice(message({ idMessage: 11, body: 'Does Continue keep it too?' }))
        );
        fixture.detectChanges();

        expect(bodies()).toEqual(['Where is the worktree removed?', 'Does Continue keep it too?']);
        expect(host.querySelector('[data-testid="chat-message-pending"]')).toBeNull();
    });

    // Confirming the same stored message twice must not swallow a second,
    // identical message that is still on its way.
    it('keeps the second of two identical messages pending until it is confirmed', () => {
        const first$ = new Subject<AgentConversationMessage>();
        const second$ = new Subject<AgentConversationMessage>();
        api['insertMessage$'].mockReturnValueOnce(first$).mockReturnValueOnce(second$);
        const host = render({ idConversation: '4' });

        send('ok');
        send('ok');
        const first = message({ idMessage: 11, body: 'ok' });
        conversationNotices.next(conversationNotice(first));
        first$.next(first);
        fixture.detectChanges();

        expect(bodies()).toEqual(['Where is the worktree removed?', 'ok', 'ok']);
        expect(host.querySelectorAll('[data-testid="chat-message-pending"]')).toHaveLength(1);
    });

    it('labels a message that is still being sent', () => {
        api['insertMessage$'].mockReturnValue(new Subject());
        const host = render({ idConversation: '4' });

        send('Hello?');

        expect(host.querySelector('[data-testid="chat-message-pending"]')!.textContent).toContain(
            'AGENT_CHAT.PENDING'
        );
    });

    it('puts a refused message back into the composer', () => {
        api['insertMessage$'].mockReturnValue(throwError(() => ({ status: 422, error: {} })));
        render({ idConversation: '4' });

        send('Hello?');

        expect(editor().written).toBe('Hello?');
    });

    it('shows each conversation cost in the list only when it is known', () => {
        api['load$'].mockReturnValue(
            of([
                { ...listItem(4, 'Priced'), costUsd: 0.41 },
                { ...listItem(5, 'Estimated'), costUsd: 0.2, costEstimated: true },
                listItem(6, 'Unpriced')
            ])
        );
        const host = render();

        const rows = Array.from(host.querySelectorAll('[data-testid="chat-list-item"]'));
        const cost = (row: Element) =>
            row.querySelector('[data-testid="chat-list-item-cost"]')?.textContent.trim() ?? null;
        expect(rows.map(cost)).toEqual(['$0.41', '≈ $0.20', null]);
        expect(rows[2].textContent).not.toContain('$');
    });

    it('marks pinned conversations in the list only', () => {
        api['load$'].mockReturnValue(
            of([
                { ...listItem(4, 'Pinned'), pinnedAt: '2026-11-16T11:00:00Z' },
                listItem(5, 'Loose')
            ])
        );
        const host = render();

        const rows = Array.from(host.querySelectorAll('[data-testid="chat-list-item"]'));
        expect(
            rows.map(row => !!row.querySelector('[data-testid="chat-list-item-pinned"]'))
        ).toEqual([true, false]);
        expect(titles(host)).toEqual(['Pinned', 'Loose']);
    });

    it('pins a conversation from the list and reloads the list in its new order', () => {
        const host = render();
        api['load$'].mockReturnValue(
            of([
                { ...listItem(5, 'Elsewhere'), pinnedAt: '2026-11-16T11:00:00Z' },
                listItem(4, 'In rurdesk')
            ])
        );

        host.querySelectorAll<HTMLElement>('[data-testid="chat-list-item-pin"]')[1].click();
        fixture.detectChanges();

        expect(api['setPinned$']).toHaveBeenCalledWith(5, true);
        expect(titles(host)).toEqual(['Elsewhere', 'In rurdesk']);
        expect(unreadStore.notifyChanged).toHaveBeenCalled();
    });

    it('unpins a pinned conversation from the list', () => {
        api['load$'].mockReturnValue(
            of([{ ...listItem(4, 'Pinned'), pinnedAt: '2026-11-16T11:00:00Z' }])
        );
        const host = render();

        host.querySelector<HTMLElement>('[data-testid="chat-list-item-pin"]')!.click();

        expect(api['setPinned$']).toHaveBeenCalledWith(4, false);
    });

    it('pins the open conversation from its header and refreshes the header and the list', () => {
        render({ idConversation: '4' });
        const callsBefore = api['load$'].mock.calls.length;
        const header = fixture.debugElement
            .query(debug => debug.name === 'app-conversation-header')
            .injector.get(ConversationHeaderStub);

        header.pinChange.emit(true);

        expect(api['setPinned$']).toHaveBeenCalledWith(4, true);
        expect(api['loadOne$']).toHaveBeenCalledTimes(2);
        expect(api['load$'].mock.calls.length).toBe(callsBefore + 1);
        expect(unreadStore.notifyChanged).toHaveBeenCalled();
    });

    it('changes the model from the header and shows the stored one', () => {
        render({ idConversation: '4' });
        api['loadOne$'].mockReturnValue(of(detail({ model: 'anthropic/claude-opus' })));
        const header = () =>
            fixture.debugElement
                .query(debug => debug.name === 'app-conversation-header')
                .injector.get(ConversationHeaderStub);

        header().settingsChange.emit({ model: 'anthropic/claude-opus' });
        fixture.detectChanges();

        expect(api['updateSettings$']).toHaveBeenCalledWith(4, {
            model: 'anthropic/claude-opus'
        });
        expect(header().detail()!.conversation.model).toBe('anthropic/claude-opus');
    });

    it('reloads the header when the change is refused', () => {
        render({ idConversation: '4' });
        api['updateSettings$'].mockReturnValue(
            throwError(() => ({ status: 409, error: { code: 'AGENT_CONVERSATION_BUSY' } }))
        );
        const header = fixture.debugElement
            .query(debug => debug.name === 'app-conversation-header')
            .injector.get(ConversationHeaderStub);

        header.settingsChange.emit({ model: null });

        expect(api['loadOne$']).toHaveBeenCalledTimes(2);
    });

    function deleteButtons(): UiConfirmStub[] {
        return fixture.debugElement
            .queryAll(debug => debug.attributes['data-testid'] === 'chat-list-item-delete')
            .map(debug => debug.injector.get(UiConfirmStub));
    }

    it('asks before deleting and changes nothing until the owner confirms', () => {
        const navigate = vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
        const host = render({ idConversation: '4' });
        const callsBefore = api['load$'].mock.calls.length;

        host.querySelector<HTMLElement>('[data-testid="chat-list-item-delete"]')!.click();

        expect(deleteButtons()[0].confirmText()).toBe('AGENT_CHAT.DELETE_CONFIRM');
        expect(api['delete$']).not.toHaveBeenCalled();
        expect(navigate).not.toHaveBeenCalled();
        expect(api['load$'].mock.calls.length).toBe(callsBefore);
        expect(unreadStore.notifyChanged).not.toHaveBeenCalled();
    });

    it('deletes the open conversation, leaves it and refreshes the list and the menu', () => {
        const navigate = vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
        render({ idConversation: '4' }, { idProject: '7' });
        const callsBefore = api['load$'].mock.calls.length;

        deleteButtons()[0].confirmed.emit();

        expect(api['delete$']).toHaveBeenCalledWith(4);
        expect(navigate).toHaveBeenCalledWith(['/agent-chat'], { queryParams: { idProject: 7 } });
        expect(api['load$'].mock.calls.length).toBe(callsBefore + 1);
        expect(unreadStore.notifyChanged).toHaveBeenCalled();
    });

    it('deletes the open conversation from its header', () => {
        const navigate = vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
        render({ idConversation: '4' });
        const header = fixture.debugElement
            .query(debug => debug.name === 'app-conversation-header')
            .injector.get(ConversationHeaderStub);

        header.deleteConversation.emit();

        expect(api['delete$']).toHaveBeenCalledWith(4);
        expect(navigate).toHaveBeenCalledWith(['/agent-chat'], {
            queryParams: { idProject: null }
        });
    });

    it('stays on the open conversation when another one is deleted', () => {
        const navigate = vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
        render({ idConversation: '4' });
        const callsBefore = api['load$'].mock.calls.length;

        deleteButtons()[1].confirmed.emit();

        expect(api['delete$']).toHaveBeenCalledWith(5);
        expect(navigate).not.toHaveBeenCalled();
        expect(api['load$'].mock.calls.length).toBe(callsBefore + 1);
        expect(unreadStore.notifyChanged).toHaveBeenCalled();
    });

    it('keeps the conversation and the page when the delete fails', () => {
        const navigate = vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
        api['delete$'].mockReturnValue(throwError(() => new Error('boom')));
        render({ idConversation: '4' });

        deleteButtons()[0].confirmed.emit();

        expect(navigate).not.toHaveBeenCalled();
        expect(unreadStore.notifyChanged).not.toHaveBeenCalled();
    });

    it('ignores late notices of a conversation it just deleted', () => {
        vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
        render({ idConversation: '4' });
        deleteButtons()[0].confirmed.emit();
        api['loadOne$'].mockClear();

        conversationNotices.next(
            conversationNotice(
                message({
                    idMessage: 12,
                    author: AgentConversationAuthor.Agent,
                    idUser: 9,
                    idTask: 3
                })
            )
        );
        thinkingNotices.next({
            payload: { idRun: 30, idTask: 3, stage: 'chat', seq: 1, events: [], idConversation: 4 }
        });
        fixture.detectChanges();

        expect(api['loadOne$']).not.toHaveBeenCalled();
        expect(api['markRead$']).not.toHaveBeenCalled();
    });

    it('a failed delete of the open conversation reloads its detail', () => {
        vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
        api['delete$'].mockReturnValue(throwError(() => new Error('boom')));
        render({ idConversation: '4' });
        api['loadOne$'].mockClear();

        deleteButtons()[0].confirmed.emit();

        expect(api['loadOne$']).toHaveBeenCalledWith(4);
    });

    it('sends nothing for a second confirm while the delete is pending', () => {
        vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
        api['delete$'].mockReturnValue(new Subject<void>());
        render({ idConversation: '4' });

        deleteButtons()[0].confirmed.emit();
        deleteButtons()[0].confirmed.emit();

        expect(api['delete$']).toHaveBeenCalledTimes(1);
    });

    it('tells assistive technology which conversations are unread', () => {
        api['load$'].mockReturnValue(of([{ ...listItem(4, 'In rurdesk'), isUnread: true }]));
        const host = render();

        expect(host.querySelector('[data-testid="chat-list-item-unread"]')!.textContent).toContain(
            'AGENT_CHAT.UNREAD'
        );
    });
    function isCentred(element: Element): boolean {
        const style = getComputedStyle(element);
        return (
            style.display === 'flex' &&
            style.alignItems === 'center' &&
            style.justifyContent === 'center' &&
            style.flexGrow === '1'
        );
    }

    it('centres the empty list message in the list pane', () => {
        api['load$'].mockReturnValue(of([]));
        const host = render();

        const empty = host.querySelector('[data-testid="chat-list-empty"]')!;
        expect(empty.textContent).toContain('AGENT_CHAT.EMPTY_LIST');
        expect(isCentred(empty)).toBe(true);
    });

    it('centres the placeholder in the content pane until a conversation is picked', () => {
        const host = render();

        const placeholder = host.querySelector('[data-testid="agent-chat-placeholder"]')!;
        expect(placeholder.textContent).toContain('AGENT_CHAT.SELECT');
        expect(isCentred(placeholder)).toBe(true);
    });

    it('offers a new conversation as a secondary icon action beside the list title', () => {
        render();

        const action = fixture.debugElement
            .query(debug => debug.attributes['data-testid'] === 'chat-list-new')
            .injector.get(UiButtonStub);
        expect(action.severity()).toBe('secondary');
        expect(action.variant()).toBe('text');
        expect(action.isIconOnly()).toBe(true);
        expect(action.ariaLabel()).toBe('AGENT_CHAT.NEW');
    });

    it('keeps the list pane at its width however long a title is', () => {
        api['load$'].mockReturnValue(of([listItem(4, 'A very long title '.repeat(20))]));
        const host = render();
        const list = host.querySelector<HTMLElement>('.agent-chat-list')!;

        expect(list.getBoundingClientRect().width).toBe(17 * 16);
    });

    function rowOf(element: Element): HTMLElement {
        return element.closest<HTMLElement>('[data-testid="chat-row"]')!;
    }

    it('shows the live thinking as a row of the agent', () => {
        api['loadOne$'].mockReturnValue(
            of(detail({}, { state: AgentStatusState.Busy, stage: 'chat', idTask: 9 }))
        );
        render({ idConversation: '4' });

        const live = fixture.debugElement.query(debug => debug.name === 'app-chat-thinking');
        const content = (live.nativeElement as HTMLElement).closest(
            '[data-testid="chat-row-content"]'
        );
        expect(content).not.toBeNull();
        const row = rowOf(content!);
        expect(row.querySelector('[data-testid="chat-row-author"]')!.textContent).toContain(
            'Agent 003'
        );
        const avatar = fixture.debugElement.query(
            debug => debug.name === 'app-avatar' && row.contains(debug.nativeElement as Node)
        );
        expect(avatar.injector.get(AvatarStub).name()).toBe('Agent 003');
    });

    describe('thinking of an answer', () => {
        const reply = (overrides: Partial<AgentConversationMessage> = {}) =>
            message({
                idMessage: 12,
                author: AgentConversationAuthor.Agent,
                idUser: 9,
                idTask: 3,
                hasThinking: true,
                body: 'In gc.go',
                ...overrides
            });

        function pills(): HTMLButtonElement[] {
            return Array.from(
                (fixture.nativeElement as HTMLElement).querySelectorAll<HTMLButtonElement>(
                    '[data-testid="chat-thinking-toggle"]'
                )
            );
        }

        function finished(): ChatThinkingStub[] {
            return fixture.debugElement
                .queryAll(debug => debug.name === 'app-chat-thinking')
                .map(debug => debug.injector.get(ChatThinkingStub))
                .filter(stub => !stub.isLive());
        }

        function toggle(): void {
            pills()[0].click();
            fixture.detectChanges();
        }

        // The turn streams while the agent answers, then its reply arrives and the turn ends.
        function finishTurn(live: LiveThinking, answer: AgentConversationMessage): void {
            api['loadOne$'].mockReturnValue(
                of(detail({}, { state: AgentStatusState.Busy, stage: 'chat', idTask: 3 }))
            );
            render({ idConversation: '4' });
            liveBlock()!.thinkingChange.emit(live);
            api['loadOne$'].mockReturnValue(of(detail()));
            conversationNotices.next(conversationNotice(answer));
            fixture.detectChanges();
        }

        it('keeps it behind a collapsed pill and fetches it when first opened', () => {
            api['loadMessages$'].mockReturnValue(of([message({}), reply()]));
            render({ idConversation: '4' });

            expect(pills()).toHaveLength(1);
            expect(pills()[0].getAttribute('aria-expanded')).toBe('false');
            expect(finished()).toHaveLength(0);
            expect(api['loadThinking$']).not.toHaveBeenCalled();

            toggle();

            expect(api['loadThinking$']).toHaveBeenCalledWith(4, 3);
            expect(pills()[0].getAttribute('aria-expanded')).toBe('true');
            expect(finished()[0].events()).toEqual([thought('reading gc.go')]);
            const content = (
                fixture.debugElement.query(debug => debug.name === 'app-chat-thinking')
                    .nativeElement as HTMLElement
            ).closest('app-message-view');
            expect(content).not.toBeNull();

            toggle();

            expect(finished()).toHaveLength(0);
            expect(pills()[0].getAttribute('aria-expanded')).toBe('false');

            toggle();

            expect(finished()[0].events()).toEqual([thought('reading gc.go')]);
            expect(api['loadThinking$']).toHaveBeenCalledTimes(1);
        });

        it('tries again on the next open after a failed load', () => {
            api['loadMessages$'].mockReturnValue(of([message({}), reply()]));
            api['loadThinking$'].mockReturnValueOnce(throwError(() => new Error('offline')));
            render({ idConversation: '4' });

            toggle();
            toggle();
            toggle();

            expect(api['loadThinking$']).toHaveBeenCalledTimes(2);
            expect(finished()[0].events()).toEqual([thought('reading gc.go')]);
        });

        it('says the stored thinking is loading until it arrives', () => {
            const stored$ = new Subject<AgentThinkingRes>();
            api['loadMessages$'].mockReturnValue(of([message({}), reply()]));
            api['loadThinking$'].mockReturnValue(stored$);
            render({ idConversation: '4' });

            toggle();

            expect(finished()[0].status()).toBe('loading');

            stored$.next(storedThinking([thought('reading gc.go')]));
            fixture.detectChanges();

            expect(finished()[0].status()).toBe('ready');
            expect(finished()[0].events()).toEqual([thought('reading gc.go')]);
        });

        it('says no thinking was recorded and asks again on the next open', () => {
            api['loadMessages$'].mockReturnValue(of([message({}), reply()]));
            api['loadThinking$'].mockReturnValue(of(storedThinking([])));
            render({ idConversation: '4' });

            toggle();

            expect(finished()[0].status()).toBe('empty');

            toggle();
            toggle();

            expect(api['loadThinking$']).toHaveBeenCalledTimes(2);
        });

        it('says the thinking could not be loaded and asks again on the next open', () => {
            api['loadMessages$'].mockReturnValue(of([message({}), reply()]));
            api['loadThinking$'].mockReturnValueOnce(throwError(() => new Error('offline')));
            render({ idConversation: '4' });

            toggle();

            expect(finished()[0].status()).toBe('failed');

            toggle();
            toggle();

            expect(finished()[0].status()).toBe('ready');
            expect(api['loadThinking$']).toHaveBeenCalledTimes(2);
        });

        it('offers no pill for an answer without thinking', () => {
            api['loadMessages$'].mockReturnValue(of([message({}), reply({ hasThinking: false })]));
            render({ idConversation: '4' });

            expect(pills()).toHaveLength(0);
        });

        it('offers the pill of a task draft with thinking', () => {
            api['loadMessages$'].mockReturnValue(
                of([
                    message({}),
                    reply({ kind: AgentConversationMessageKind.TaskDraft, body: '# Draft' })
                ])
            );
            render({ idConversation: '4' });

            expect(rowOf(pills()[0]).querySelector('app-task-draft-card')).not.toBeNull();

            toggle();

            expect(finished()[0].events()).toEqual([thought('reading gc.go')]);
        });

        it('shows what streamed for the turn when its reply arrives, without fetching it', () => {
            finishTurn(
                { idTask: 3, events: [thought('streamed')], hasGap: false },
                reply({ hasThinking: false })
            );

            expect(liveBlock()).toBeNull();
            expect(pills()).toHaveLength(1);
            expect(pills()[0].getAttribute('aria-expanded')).toBe('false');

            toggle();

            expect(finished()[0].events()).toEqual([thought('streamed')]);
            expect(api['loadThinking$']).not.toHaveBeenCalled();
        });

        it('fetches the thinking of a turn that lost some of its stream when opened', () => {
            finishTurn({ idTask: 3, events: [thought('third')], hasGap: true }, reply());

            toggle();

            expect(api['loadThinking$']).toHaveBeenCalledWith(4, 3);
            expect(finished()[0].events()).toEqual([thought('reading gc.go')]);
        });

        it('offers no pill when the turn streamed nothing and the reply has no thinking', () => {
            finishTurn(
                { idTask: 2, events: [thought('other turn')], hasGap: false },
                reply({ hasThinking: false })
            );

            expect(pills()).toHaveLength(0);
        });
    });

    it("shows a task draft in the agent message's content column", () => {
        api['loadMessages$'].mockReturnValue(
            of([
                message({}),
                message({
                    idMessage: 12,
                    author: AgentConversationAuthor.Agent,
                    idUser: 9,
                    kind: AgentConversationMessageKind.TaskDraft,
                    body: '# Keep the worktree'
                })
            ])
        );
        render({ idConversation: '4' });

        const card = fixture.debugElement.query(debug => debug.name === 'app-task-draft-card');
        const content = (card.nativeElement as HTMLElement).closest(
            '[data-testid="chat-row-content"]'
        );
        expect(content).not.toBeNull();
        expect(
            rowOf(content!).querySelector('[data-testid="chat-row-author"]')!.textContent
        ).toContain('Agent 003');
    });
    describe('at phone width', () => {
        const overflows = (element: HTMLElement) => element.scrollWidth > element.clientWidth;

        beforeEach(async () => {
            await page.viewport(390, 844);
        });

        function visible(host: HTMLElement, selector: string): boolean {
            const element = host.querySelector<HTMLElement>(selector);
            return element !== null && element.getBoundingClientRect().width > 0;
        }

        it('shows only the list, full width, while no conversation is open', () => {
            const host = render();

            expect(visible(host, '.agent-chat-list')).toBe(true);
            expect(visible(host, '.agent-chat-main')).toBe(false);
            expect(host.querySelector('.agent-chat-list')!.getBoundingClientRect().width).toBe(
                host.getBoundingClientRect().width
            );
            expect(overflows(host)).toBe(false);
        });

        it('shows only the open conversation and goes back to the list from its header', () => {
            const navigate = vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
            const host = render({ idConversation: '4' }, { idProject: '7' });

            expect(visible(host, '.agent-chat-list')).toBe(false);
            expect(visible(host, '.agent-chat-main')).toBe(true);
            expect(overflows(host)).toBe(false);

            fixture.debugElement
                .query(debug => debug.name === 'app-conversation-header')
                .injector.get(ConversationHeaderStub)
                .back.emit();

            expect(navigate).toHaveBeenCalledWith(['/agent-chat'], {
                queryParams: { idProject: 7 }
            });
        });

        it('shows only the new conversation form and goes back to the list from it', () => {
            const navigate = vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
            const host = render({}, { new: '1', idProject: '7' });

            expect(visible(host, '.agent-chat-list')).toBe(false);
            expect(visible(host, '.agent-chat-main')).toBe(true);

            fixture.debugElement
                .query(debug => debug.name === 'app-new-conversation')
                .injector.get(NewConversationStub)
                .back.emit();

            expect(navigate).toHaveBeenCalledWith(['/agent-chat'], {
                queryParams: { idProject: 7 }
            });
        });

        it('keeps a long title, long answers and code inside the width', () => {
            api['load$'].mockReturnValue(of([listItem(4, 'A very long title '.repeat(20))]));
            const host = render();
            expect(overflows(host)).toBe(false);
            expect(overflows(host.querySelector<HTMLElement>('.conversation-list-items')!)).toBe(
                false
            );
        });

        it('keeps the list actions inside each conversation, reachable without hover', () => {
            const host = render();

            const actions = host.querySelector<HTMLElement>('.conversation-list-item-actions')!;
            expect(getComputedStyle(actions).opacity).toBe('1');
            expectInsideBorder(host);
        });
    });

    describe('at desktop width', () => {
        beforeEach(async () => {
            await page.viewport(1440, 900);
        });

        afterEach(async () => {
            await page.viewport(1280, 800);
        });

        it('keeps the list actions inside each conversation, clear of its border', () => {
            const host = render({ idConversation: '4' });

            expectInsideBorder(host);
            const actions = host.querySelector<HTMLElement>('.conversation-list-item-actions')!;
            expect(getComputedStyle(actions).backgroundColor).toBe('rgba(0, 0, 0, 0)');
        });

        it('shows the list and the conversation side by side', () => {
            const host = render({ idConversation: '4' });

            const list = host.querySelector<HTMLElement>('.agent-chat-list')!;
            const main = host.querySelector<HTMLElement>('.agent-chat-main')!;
            expect(list.getBoundingClientRect().width).toBe(17 * 16);
            expect(main.getBoundingClientRect().left).toBe(list.getBoundingClientRect().right);
        });
    });
});

@Component({
    selector: 'app-message-body',
    template: '<p>{{ text }}</p>',
    // Like the real body, whose host takes no box of its own.
    styles: ':host { display: contents; }',
    standalone: true
})
class WideBodyStub {
    protected readonly text = 'A long answer that wraps over several lines. '.repeat(20);
}

describe('AgentChatPage message rows (browser, real rows)', () => {
    let fixture: ComponentFixture<AgentChatPage>;

    beforeEach(async () => {
        await page.viewport(390, 844);
        const api = {
            load$: vi.fn(() => of([])),
            loadOne$: vi.fn(() => of(detail())),
            loadMessages$: vi.fn(() =>
                of([
                    message({}),
                    message({
                        idMessage: 11,
                        author: AgentConversationAuthor.Agent,
                        idUser: 9,
                        idTask: 3,
                        kind: AgentConversationMessageKind.Text,
                        body: 'Hi'
                    }),
                    message({
                        idMessage: 12,
                        author: AgentConversationAuthor.Agent,
                        idUser: 9,
                        kind: AgentConversationMessageKind.TaskDraft,
                        body: '# Keep the worktree'
                    })
                ])
            ),
            markRead$: vi.fn(() => of(undefined)),
            loadAgents$: vi.fn(() => of([agent]))
        };
        await TestBed.configureTestingModule({
            declarations: [
                AgentChatPage,
                ConversationListComponent,
                ChatRowComponent,
                MessageViewComponent,
                UiButtonComponent
            ],
            imports: [
                TranslateModule.forRoot(),
                ReactiveFormsModule,
                RouterModule,
                WideBodyStub,
                MessageEditorStub,
                ConversationHeaderStub,
                NewConversationStub,
                ChatThinkingStub,
                TaskDraftCardStub,
                TaskDraftDialogStub,
                AvatarStub,
                TablerIconStub,
                UiChoiceStub,
                UiConfirmStub,
                UiTooltipStub
            ],
            providers: [
                provideRouter([]),
                {
                    provide: ActivatedRoute,
                    useValue: {
                        paramMap: of(convertToParamMap({ idConversation: '4' })),
                        queryParamMap: of(convertToParamMap({}))
                    }
                },
                { provide: AgentConversationApi, useValue: api },
                { provide: ProjectApi, useValue: { load$: () => of([Fixtures.project()]) } },
                { provide: UiToastService, useValue: { show: vi.fn() } },
                { provide: AuthStore, useValue: { getUser: () => owner } },
                {
                    provide: AgentConversationUnreadStore,
                    useValue: { markRead: vi.fn(), notifyChanged: vi.fn() }
                },
                {
                    provide: NoticeService,
                    useValue: {
                        agentConversation$: new Subject(),
                        agentConversationTurn$: new Subject(),
                        agentThinking$: new Subject()
                    }
                }
            ]
        }).compileComponents();
        TestBed.inject(TranslateService).use('en');
        withSpacingTokens();
        fixture = TestBed.createComponent(AgentChatPage);
        fixture.detectChanges();
    });

    afterEach(async () => {
        await page.viewport(1280, 800);
    });

    function host(): HTMLElement {
        return fixture.nativeElement as HTMLElement;
    }

    // Owner, then an answer and a draft of the agent that continue each other.
    function gaps(): { betweenAuthors: number; withinAuthor: number } {
        const entries = Array.from(host().querySelectorAll<HTMLElement>('.agent-chat-entry')).map(
            entry => entry.getBoundingClientRect()
        );
        return {
            betweenAuthors: entries[1].top - entries[0].bottom,
            withinAuthor: entries[2].top - entries[1].bottom
        };
    }

    it('sets the messages of different authors clearly apart on a phone', () => {
        const { betweenAuthors, withinAuthor } = gaps();

        expect(betweenAuthors).toBeGreaterThanOrEqual(24);
        expect(betweenAuthors).toBeGreaterThan(withinAuthor + 16);
    });

    it('keeps the author line of a message close to its text on a phone', () => {
        const views = host().querySelectorAll<HTMLElement>('app-message-view .message-view');
        const authorToText = (author: Element, text: Element): number =>
            text.getBoundingClientRect().top - author.getBoundingClientRect().bottom;

        views.forEach(view => {
            expect(
                authorToText(
                    view.querySelector('.message-meta')!,
                    view.querySelector('.message-text')!
                )
            ).toBeLessThanOrEqual(14);
        });
    });

    it('sets the messages of different authors apart on a desktop', async () => {
        await page.viewport(1280, 800);
        fixture.detectChanges();

        const { betweenAuthors, withinAuthor } = gaps();

        expect(betweenAuthors).toBeGreaterThanOrEqual(16);
        expect(betweenAuthors).toBeGreaterThan(withinAuthor + 8);
    });

    it('gives the text of an answer the whole width but the gutters', () => {
        const views = host().querySelectorAll<HTMLElement>('app-message-view .message-view');
        const answer = views[1];
        const text = answer.querySelector<HTMLElement>('.message-text')!.getBoundingClientRect();

        expect(text.width).toBeGreaterThanOrEqual(330);
        expect(text.right).toBeLessThanOrEqual(390 - 16 + 1);
        expect(host().scrollWidth).toBeLessThanOrEqual(host().clientWidth);
    });

    it('puts a small avatar beside the author instead of a column that insets the text', () => {
        const view = host().querySelector<HTMLElement>('app-message-view .message-view')!;
        const avatar = view.querySelector<HTMLElement>('.message-avatar')!.getBoundingClientRect();
        const author = view.querySelector<HTMLElement>('.message-meta')!.getBoundingClientRect();
        const text = view.querySelector<HTMLElement>('.message-text')!.getBoundingClientRect();

        expect(avatar.width).toBeLessThanOrEqual(28);
        expect(author.left).toBeGreaterThanOrEqual(avatar.right);
        expect(text.left).toBeLessThan(avatar.right);
        expect(text.top).toBeGreaterThanOrEqual(avatar.bottom - 1);
    });

    it('gives the rows of the agent the same full width', () => {
        const content = host().querySelector<HTMLElement>(
            '[data-testid="chat-row-content"] app-task-draft-card'
        )!;

        expect(content.getBoundingClientRect().width).toBeGreaterThanOrEqual(330);
    });

    it('names the Create task button for assistive technology on its inner button', () => {
        const button = host().querySelector<HTMLElement>(
            '[data-testid="chat-create-task"] button'
        )!;

        expect(button.getAttribute('aria-label')).toBe('AGENT_CHAT.CREATE_TASK');
    });

    it('keeps the touch target of Create task without growing the author line', () => {
        const view = host().querySelectorAll<HTMLElement>('app-message-view .message-view')[1];
        const action = view.querySelector<HTMLElement>('[data-testid="chat-create-task"] button')!;
        const style = getComputedStyle(view);
        const text = view.querySelector<HTMLElement>('.message-text')!;
        const authorLine =
            text.getBoundingClientRect().top -
            view.getBoundingClientRect().top -
            parseFloat(style.paddingTop) -
            parseFloat(style.rowGap) -
            parseFloat(getComputedStyle(text).marginTop);

        expect(action.getBoundingClientRect().height).toBeGreaterThanOrEqual(44);
        expect(authorLine).toBeLessThanOrEqual(32);
    });

    it('gives Create task a square button on a phone and on a desktop', async () => {
        const button = () =>
            host()
                .querySelector<HTMLElement>('[data-testid="chat-create-task"] button')!
                .getBoundingClientRect();

        expect(button().width).toBeCloseTo(button().height, 0);
        expect(button().height).toBeGreaterThanOrEqual(44);

        await page.viewport(1280, 800);
        fixture.detectChanges();

        expect(button().width).toBeGreaterThan(0);
        expect(button().width).toBeCloseTo(button().height, 0);
    });

    it('keeps the side by side message layout on a wide screen', async () => {
        await page.viewport(1280, 800);

        const view = host().querySelector<HTMLElement>('app-message-view .message-view')!;
        expect(getComputedStyle(view).display).toBe('flex');
    });
});

describe('AgentChatPage thinking of answers (browser, real rows)', () => {
    let fixture: ComponentFixture<AgentChatPage>;
    let api: Record<string, ReturnType<typeof vi.fn>>;
    let conversationNotices: Subject<Notice<AgentConversationMessage>>;
    let turnNotices: Subject<Notice<AgentConversationTurnNotice>>;
    let thinkingNotices: Subject<{ payload: AgentThinkingNotice }>;

    const reply = (overrides: Partial<AgentConversationMessage> = {}) =>
        message({
            idMessage: 11,
            author: AgentConversationAuthor.Agent,
            idUser: 9,
            idTask: 3,
            hasThinking: true,
            body: 'In gc.go',
            createdAt: '2026-11-16T09:01:00Z',
            ...overrides
        });

    beforeEach(async () => {
        await page.viewport(1280, 800);
        conversationNotices = new Subject();
        turnNotices = new Subject();
        thinkingNotices = new Subject();
        api = {
            load$: vi.fn(() => of([])),
            loadOne$: vi.fn(() => of(detail())),
            loadMessages$: vi.fn(() => of([message({})])),
            markRead$: vi.fn(() => of(undefined)),
            loadAgents$: vi.fn(() => of([agent])),
            loadThinking$: vi.fn(() => of(storedThinking([thought('reading gc.go')])))
        };
        await TestBed.configureTestingModule({
            declarations: [
                AgentChatPage,
                ConversationListComponent,
                ChatRowComponent,
                ChatThinkingComponent,
                ChatThinkingPillComponent,
                AgentThinkingLinesComponent,
                MessageViewComponent,
                UiButtonComponent
            ],
            imports: [
                TranslateModule.forRoot(),
                ReactiveFormsModule,
                RouterModule,
                WideBodyStub,
                MessageEditorStub,
                ConversationHeaderStub,
                NewConversationStub,
                TaskDraftCardStub,
                TaskDraftDialogStub,
                AvatarStub,
                TablerIconStub,
                UiChoiceStub,
                UiConfirmStub,
                UiTooltipStub
            ],
            providers: [
                provideRouter([]),
                {
                    provide: ActivatedRoute,
                    useValue: {
                        paramMap: of(convertToParamMap({ idConversation: '4' })),
                        queryParamMap: of(convertToParamMap({}))
                    }
                },
                { provide: AgentConversationApi, useValue: api },
                { provide: ProjectApi, useValue: { load$: () => of([Fixtures.project()]) } },
                { provide: UiToastService, useValue: { show: vi.fn() } },
                { provide: AuthStore, useValue: { getUser: () => owner } },
                {
                    provide: AgentConversationUnreadStore,
                    useValue: { markRead: vi.fn(), notifyChanged: vi.fn() }
                },
                {
                    provide: NoticeService,
                    useValue: {
                        agentConversation$: conversationNotices,
                        agentConversationTurn$: turnNotices,
                        agentThinking$: thinkingNotices
                    }
                }
            ]
        }).compileComponents();
        const translate = TestBed.inject(TranslateService);
        translate.setTranslation('en', { AGENT_CHAT: { THINKING: { LABEL: 'Thinking' } } });
        translate.use('en');
        withSpacingTokens();
    });

    afterEach(async () => {
        await page.viewport(1280, 800);
    });

    function render(messages: AgentConversationMessage[]): HTMLElement {
        api['loadMessages$'].mockReturnValue(of(messages));
        fixture = TestBed.createComponent(AgentChatPage);
        fixture.detectChanges();
        return fixture.nativeElement as HTMLElement;
    }

    function pill(scope: Element): HTMLButtonElement | null {
        return scope.querySelector<HTMLButtonElement>('[data-testid="chat-thinking-toggle"]');
    }

    function click(button: HTMLButtonElement): void {
        button.click();
        fixture.detectChanges();
    }

    function lines(scope: Element): HTMLElement | null {
        return scope.querySelector<HTMLElement>('[data-testid="agent-thinking-body"]');
    }

    function rect(element: Element): DOMRect {
        return element.getBoundingClientRect();
    }

    function isOnLineWith(element: Element, other: Element): boolean {
        const a = rect(element);
        const b = rect(other);
        const middle = (b.top + b.bottom) / 2;
        return a.top <= middle && a.bottom >= middle;
    }

    function answerView(host: HTMLElement, index = 1): HTMLElement {
        return host.querySelectorAll<HTMLElement>('app-message-view .message-view')[index];
    }

    it('shows the thinking of an answer behind a pill beside its name and time', () => {
        const host = render([message({}), reply()]);
        const view = answerView(host);
        const meta = view.querySelector('.message-meta')!;
        const toggle = pill(view)!;

        expect(meta.contains(toggle)).toBe(true);
        expect(rect(toggle).left).toBeGreaterThanOrEqual(
            rect(meta.querySelector('.message-date')!).right
        );
        expect(isOnLineWith(toggle, meta.querySelector('.message-author')!)).toBe(true);
        expect(toggle.getAttribute('aria-expanded')).toBe('false');
        expect(lines(view)).toBeNull();

        click(toggle);

        const opened = lines(view)!;
        expect(toggle.getAttribute('aria-expanded')).toBe('true');
        expect(opened.textContent).toContain('reading gc.go');
        expect(rect(opened).top).toBeGreaterThanOrEqual(rect(meta).bottom);
        expect(rect(opened).bottom).toBeLessThanOrEqual(
            rect(view.querySelector('.message-text p')!).top
        );

        click(toggle);

        expect(lines(view)).toBeNull();
        expect(toggle.getAttribute('aria-expanded')).toBe('false');
    });

    it('shows the thinking of a task draft behind a pill beside its name and time', () => {
        const host = render([
            message({}),
            reply({ kind: AgentConversationMessageKind.TaskDraft, body: '# Draft' })
        ]);
        const row = host.querySelector<HTMLElement>('[data-testid="chat-row"]')!;
        const meta = row.querySelector('.chat-row-meta')!;
        const toggle = pill(row)!;

        expect(meta.contains(toggle)).toBe(true);
        expect(isOnLineWith(toggle, meta.querySelector('.chat-row-author')!)).toBe(true);
        expect(lines(row)).toBeNull();

        click(toggle);

        const opened = lines(row)!;
        expect(opened.textContent).toContain('reading gc.go');
        expect(rect(opened).top).toBeGreaterThanOrEqual(rect(meta).bottom);
        expect(rect(opened).bottom).toBeLessThanOrEqual(
            rect(row.querySelector('app-task-draft-card')!).top
        );

        click(toggle);

        expect(lines(row)).toBeNull();
    });

    it('puts the pill of an answer that continues the previous one before its text', () => {
        const host = render([
            message({}),
            reply({ idTask: 2, hasThinking: false, body: 'First part' }),
            reply({ idMessage: 12, createdAt: '2026-11-16T09:02:00Z' })
        ]);
        const view = answerView(host, 2);
        const toggle = pill(view)!;

        expect(view.classList).toContain('continuation');
        expect(view.querySelector('.message-meta')).toBeNull();
        expect(rect(toggle).bottom).toBeLessThanOrEqual(
            rect(view.querySelector('.message-text p')!).top
        );
        expect(rect(toggle).top).toBeGreaterThanOrEqual(rect(view).top);
        expect(pill(answerView(host, 1))).toBeNull();
    });

    it('offers no pill for an answer without thinking', () => {
        const host = render([message({}), reply({ hasThinking: false })]);

        expect(pill(host)).toBeNull();
    });

    it('keeps the author line as tall as one without the pill', () => {
        const host = render([message({}), reply()]);
        const withPill = rect(answerView(host).querySelector('.message-meta')!).height;
        const withoutPill = rect(answerView(host, 0).querySelector('.message-meta')!).height;

        expect(pill(answerView(host))).not.toBeNull();
        expect(Math.abs(withPill - withoutPill)).toBeLessThanOrEqual(1);
    });

    function startTurn(): HTMLElement {
        api['loadOne$'].mockReturnValue(
            of(detail({}, { state: AgentStatusState.Busy, stage: 'chat' }))
        );
        return render([message({})]);
    }

    function stream(events: AgentThinkingEvent[], seq = 1): void {
        thinkingNotices.next({
            payload: { idRun: 30, idTask: 3, stage: 'chat', seq, events, idConversation: 4 }
        });
        fixture.detectChanges();
    }

    function answer(sent: AgentConversationMessage): void {
        api['loadOne$'].mockReturnValue(of(detail()));
        conversationNotices.next(conversationNotice(sent));
        fixture.detectChanges();
    }

    it('keeps the thinking of the turn that just ended for its answer without fetching it', () => {
        const host = startTurn();
        stream([thought('reading gc.go')]);
        stream([thought(' and retention.go')], 2);

        expect(lines(host.querySelector('[role="status"]')!)!.textContent).toContain(
            'reading gc.go and retention.go'
        );

        answer(reply());

        expect(host.querySelector('[role="status"]')).toBeNull();
        const toggle = pill(answerView(host))!;
        expect(toggle.getAttribute('aria-expanded')).toBe('false');

        click(toggle);

        expect(lines(answerView(host))!.textContent).toContain('reading gc.go and retention.go');
        expect(api['loadThinking$']).not.toHaveBeenCalled();
    });

    it('keeps each turn of the live block for its own answer', () => {
        const host = startTurn();
        stream([thought('first turn')]);
        thinkingNotices.next({
            payload: {
                idRun: 30,
                idTask: 4,
                stage: 'chat',
                seq: 1,
                events: [thought('second turn')],
                idConversation: 4
            }
        });
        fixture.detectChanges();

        conversationNotices.next(conversationNotice(reply()));
        fixture.detectChanges();
        click(pill(answerView(host))!);

        expect(lines(host.querySelector('[role="status"]')!)!.textContent).toContain('second turn');
        const opened = lines(answerView(host))!.textContent;
        expect(opened).toContain('first turn');
        expect(opened).not.toContain('second turn');
        expect(api['loadThinking$']).not.toHaveBeenCalled();
    });

    it('offers no pill when the turn streamed nothing and the answer has no thinking', () => {
        const host = startTurn();

        answer(reply({ hasThinking: false }));

        expect(pill(host)).toBeNull();
    });

    describe('at phone width', () => {
        beforeEach(async () => {
            await page.viewport(390, 844);
        });

        it('fits the pill into the author line beside the time', () => {
            const host = render([message({}), reply()]);
            fixture.detectChanges();
            const view = answerView(host);
            const meta = view.querySelector('.message-meta')!;
            const toggle = pill(view)!;

            expect(meta.contains(toggle)).toBe(true);
            expect(isOnLineWith(toggle, meta.querySelector('.message-date')!)).toBe(true);
            expect(rect(toggle).right).toBeLessThanOrEqual(rect(view).right);
            expect(rect(meta).height).toBeLessThanOrEqual(32);
            expect(host.scrollWidth).toBeLessThanOrEqual(host.clientWidth);
        });

        // Owner, an answer, then an answer that continues it.
        function renderContinued(continued: Partial<AgentConversationMessage>): HTMLElement {
            return render([
                message({}),
                reply({ idTask: 2, hasThinking: false, body: 'First part' }),
                reply({ idMessage: 12, createdAt: '2026-11-16T09:02:00Z', ...continued })
            ]);
        }

        function expectSharedRow(action: Element, first: Element): void {
            const middle = (rect(first).top + rect(first).bottom) / 2;
            const firstLineMiddle = rect(first).top + 12;
            const target = rect(action);
            expect(target.top).toBeLessThanOrEqual(Math.min(middle, firstLineMiddle));
            expect(target.bottom).toBeGreaterThanOrEqual(Math.min(middle, firstLineMiddle));
        }

        it('puts the pill of a continued answer on the row of its Create task, close to the answer above', () => {
            const host = renderContinued({});
            const previousText = answerView(host, 1).querySelector('.message-text')!;
            const view = answerView(host, 2);
            const toggle = pill(view)!;
            const action = view.querySelector('[data-testid="chat-create-task"] button')!;

            expect(view.classList).toContain('continuation');
            expect(rect(toggle).top - rect(previousText).bottom).toBeLessThanOrEqual(16);
            expectSharedRow(action, toggle);
            expect(rect(toggle).bottom).toBeLessThanOrEqual(
                rect(view.querySelector('.message-text')!).top
            );
            expect(host.scrollWidth).toBeLessThanOrEqual(host.clientWidth);
        });

        it('puts the text of a continued answer without thinking on the row of its Create task', () => {
            const host = renderContinued({ hasThinking: false });
            const previousText = answerView(host, 1).querySelector('.message-text')!;
            const view = answerView(host, 2);
            const text = view.querySelector('.message-text')!;
            const action = view.querySelector('[data-testid="chat-create-task"] button')!;

            expect(rect(text).top - rect(previousText).bottom).toBeLessThanOrEqual(16);
            expectSharedRow(action, text);
            expect(rect(action).left).toBeGreaterThanOrEqual(rect(text).right);
            expect(host.scrollWidth).toBeLessThanOrEqual(host.clientWidth);
        });

        it('opens the thinking of a continued answer across the width below its pill', () => {
            const host = renderContinued({});
            const view = answerView(host, 2);

            click(pill(view)!);

            const opened = lines(view)!;
            expect(rect(opened).top).toBeGreaterThanOrEqual(rect(pill(view)!).bottom);
            expect(rect(opened).bottom).toBeLessThanOrEqual(
                rect(view.querySelector('.message-text')!).top
            );
            expect(rect(opened.closest('.chat-thinking-panel')!).width).toBeGreaterThanOrEqual(330);
        });

        it('keeps the pill inside the width beside a long agent name', () => {
            api['loadOne$'].mockReturnValue(
                of({
                    ...detail(),
                    agent: Fixtures.agent({ idUser: 9, name: 'Release and migration agent 003' })
                })
            );
            const host = render([message({}), reply()]);
            const view = answerView(host);

            expect(rect(pill(view)!).right).toBeLessThanOrEqual(
                rect(view.querySelector('.message-meta')!).right
            );
            expect(host.scrollWidth).toBeLessThanOrEqual(host.clientWidth);
        });

        it('opens the thinking between the author line and the text, inside the width', () => {
            const host = render([message({}), reply()]);
            const view = answerView(host);

            click(pill(view)!);

            const opened = lines(view)!;
            expect(rect(opened).top).toBeGreaterThanOrEqual(
                rect(view.querySelector('.message-meta')!).bottom
            );
            expect(rect(opened).bottom).toBeLessThanOrEqual(
                rect(view.querySelector('.message-text')!).top
            );
            expect(rect(opened).right).toBeLessThanOrEqual(390);
            expect(host.scrollWidth).toBeLessThanOrEqual(host.clientWidth);
        });
    });
});

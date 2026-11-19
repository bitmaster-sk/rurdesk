import { ComponentFixture, TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { Subject, of, throwError } from 'rxjs';
import { AgentThinkingKind } from 'src/app/agent/constants/agent-thinking-kind.enum';
import {
    AgentThinkingEvent,
    AgentThinkingNotice,
    AgentThinkingRes
} from 'src/app/agent/model/agent-thinking.model';
import { AgentThinkingLinesComponent } from 'src/app/shared/components/agent-thinking-lines/agent-thinking-lines.component';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { TablerIconStub } from 'src/testing/stubs';
import { AgentConversationApi } from '../../api/agent-conversation.api.service';
import { LiveThinking } from '../../entity/live-thinking.entity';
import { ChatThinkingComponent } from './chat-thinking.component';

const thought = (text: string): AgentThinkingEvent => ({
    kind: AgentThinkingKind.Thinking,
    text,
    at: 1
});
const tool = (name: string, text: string): AgentThinkingEvent => ({
    kind: AgentThinkingKind.Tool,
    tool: name,
    text,
    at: 2
});

describe('ChatThinkingComponent (browser)', () => {
    let fixture: ComponentFixture<ChatThinkingComponent>;
    let notices: Subject<{ payload: AgentThinkingNotice }>;
    let loadThinking$: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        notices = new Subject();
        loadThinking$ = vi
            .fn()
            .mockReturnValue(
                of({ idRun: 30, stage: 'chat', events: [], lastSeq: 0, isComplete: false })
            );
        await TestBed.configureTestingModule({
            declarations: [ChatThinkingComponent, AgentThinkingLinesComponent],
            imports: [TranslateModule.forRoot(), TablerIconStub],
            providers: [
                { provide: NoticeService, useValue: { agentThinking$: notices } },
                { provide: AgentConversationApi, useValue: { loadThinking$ } }
            ]
        }).compileComponents();
    });

    function renderLive(idTask: number | null = null): HTMLElement {
        fixture = TestBed.createComponent(ChatThinkingComponent);
        fixture.componentRef.setInput('idConversation', 4);
        fixture.componentRef.setInput('idRun', 30);
        fixture.componentRef.setInput('idTask', idTask);
        fixture.componentRef.setInput('isLive', true);
        fixture.detectChanges();
        return fixture.nativeElement as HTMLElement;
    }

    function renderFinished(events: AgentThinkingEvent[]): HTMLElement {
        fixture = TestBed.createComponent(ChatThinkingComponent);
        fixture.componentRef.setInput('idConversation', 4);
        fixture.componentRef.setInput('events', events);
        fixture.detectChanges();
        return fixture.nativeElement as HTMLElement;
    }

    function emit(events: AgentThinkingEvent[], seq = 1, idTask = 9, idRun = 30): void {
        notices.next({
            payload: { idRun, idTask, stage: 'chat', seq, events, idConversation: 4 }
        });
        fixture.detectChanges();
    }

    function body(host: HTMLElement): HTMLElement | null {
        return host.querySelector('[data-testid="agent-thinking-body"]');
    }

    it('says the agent is thinking until the first event arrives', () => {
        const host = renderLive();

        expect(host.querySelector('[data-testid="chat-thinking-waiting"]')).not.toBeNull();

        emit([thought('reading gc.go')]);

        expect(host.querySelector('[data-testid="chat-thinking-waiting"]')).toBeNull();
        expect(body(host)!.textContent).toContain('reading gc.go');
    });

    it('appends what streams in for this run and ignores other runs', () => {
        const host = renderLive();

        emit([thought('reading gc.go')]);
        emit([thought('someone else')], 1, 50, 31);
        emit([thought(' and retention.go')], 2);

        expect(body(host)!.textContent).toContain('reading gc.go and retention.go');
        expect(body(host)!.textContent).not.toContain('someone else');
    });

    it('starts over when the next turn begins', () => {
        const host = renderLive();

        emit([thought('first turn')]);
        emit([thought('second turn')], 1, 10);

        expect(body(host)!.textContent).toContain('second turn');
        expect(body(host)!.textContent).not.toContain('first turn');
    });

    // A turn seen from its first batch has nothing to catch up on.
    it('does not fetch a turn it streams from the first batch', () => {
        const host = renderLive();

        emit([thought('from the start')]);
        emit([thought(' and on')], 2);

        expect(loadThinking$).not.toHaveBeenCalled();
        expect(body(host)!.textContent).toContain('from the start and on');
    });

    // The page knows the running task from the status or from the notice that
    // woke it, so the block replays the turn so far as soon as it appears.
    it('replays the turn so far at mount and keeps streaming after it', () => {
        loadThinking$.mockReturnValue(
            of({
                idRun: 30,
                idTask: 9,
                stage: 'chat',
                events: [thought('what came before'), tool('developer__shell', 'ls')],
                lastSeq: 4,
                isComplete: false
            })
        );
        const host = renderLive(9);

        expect(loadThinking$).toHaveBeenCalledWith(4, 9);
        emit([tool('developer__shell', 'ls')], 4);
        emit([thought('and after')], 5);

        const text = body(host)!.textContent;
        expect(text).toContain('what came before');
        expect(text.indexOf('what came before')).toBeLessThan(text.indexOf('and after'));
        expect(body(host)!.querySelectorAll('.thinking-act')).toHaveLength(1);
        expect(host.querySelector('.thinking-gap')).toBeNull();
    });

    it('catches up when it joins a turn mid-stream', () => {
        renderLive();

        emit([thought('third')], 3);

        expect(loadThinking$).toHaveBeenCalledWith(4, 9);
    });

    it('keeps what streamed while the turn so far was loading', () => {
        const stored$ = new Subject<AgentThinkingRes>();
        loadThinking$.mockReturnValue(stored$);
        const host = renderLive(9);

        emit([thought('third')], 3);
        emit([thought('fourth')], 4);
        stored$.next({
            idRun: 30,
            idTask: 9,
            stage: 'chat',
            events: [thought('first'), tool('developer__shell', 'ls'), thought('third')],
            lastSeq: 3,
            isComplete: false
        });
        fixture.detectChanges();

        const text = body(host)!.textContent;
        expect(text).toContain('first');
        expect(text).toContain('fourth');
        expect(text.split('third')).toHaveLength(2);
        expect(host.querySelector('.thinking-gap')).toBeNull();
    });

    it('announces itself to assistive technology', () => {
        const host = renderLive();

        const block = host.querySelector('[data-testid="chat-thinking"]')!;
        expect(block.getAttribute('role')).toBe('status');
        expect(block.getAttribute('aria-live')).toBe('polite');
    });

    it('shows live and finished thinking in the same panel', () => {
        const live = renderLive();
        emit([thought('reading gc.go')]);
        expect(body(live)!.closest('.chat-thinking-panel')).not.toBeNull();

        const finished = renderFinished([thought('done')]);

        expect(body(finished)!.closest('.chat-thinking-panel')).not.toBeNull();
        expect(body(finished)!.textContent).toContain('done');
    });

    it('shows the thinking and tool calls of a finished reply it is given without fetching', () => {
        const host = renderFinished([thought('checking'), tool('developer__shell', 'rg worktree')]);

        expect(loadThinking$).not.toHaveBeenCalled();
        expect(host.querySelector('[role="status"]')).toBeNull();
        const act = body(host)!.querySelector('.thinking-act')!;
        expect(act.textContent).toContain('developer__shell');
        expect(act.textContent).toContain('rg worktree');
        const icon = fixture.debugElement
            .query(
                debug =>
                    debug.name === 'tabler-icon' && !!debug.parent?.classes['thinking-act-chip']
            )
            .injector.get(TablerIconStub);
        expect(icon.icon()).toBe('terminal-2');
    });

    it.each([
        ['loading', 'chat-thinking-loading', 'AGENT_CHAT.THINKING.LOADING'],
        ['empty', 'chat-thinking-empty', 'AGENT_CHAT.THINKING.EMPTY'],
        ['failed', 'chat-thinking-failed', 'AGENT_CHAT.THINKING.FAILED']
    ])('says so in the panel while the stored thinking is %s', (status, testId, key) => {
        fixture = TestBed.createComponent(ChatThinkingComponent);
        fixture.componentRef.setInput('status', status);
        fixture.detectChanges();
        const host = fixture.nativeElement as HTMLElement;

        const line = host.querySelector(`[data-testid="${testId}"]`)!;
        expect(line.closest('.chat-thinking-panel')).not.toBeNull();
        expect(line.textContent).toContain(key);
        expect(body(host)).toBeNull();
    });

    describe('hands what streamed to the page', () => {
        let handed: LiveThinking[];

        function renderLiveHanded(idTask: number | null = null): HTMLElement {
            handed = [];
            const host = renderLive(idTask);
            fixture.componentInstance.thinkingChange.subscribe(change => handed.push(change));
            return host;
        }

        it('as the turn streams, under the turn it belongs to', () => {
            renderLiveHanded();

            emit([thought('reading gc.go')]);
            emit([thought(' and on')], 2);

            expect(handed.at(-1)).toEqual({
                idTask: 9,
                events: [thought('reading gc.go'), thought(' and on')],
                hasGap: false
            });
        });

        it('with the turn so far it replayed in front', () => {
            loadThinking$.mockReturnValue(
                of({
                    idRun: 30,
                    idTask: 9,
                    stage: 'chat',
                    events: [thought('before')],
                    lastSeq: 1,
                    isComplete: false
                })
            );
            renderLiveHanded(9);

            emit([thought('after')], 2);

            expect(handed.at(-1)).toEqual({
                idTask: 9,
                events: [thought('before'), thought('after')],
                hasGap: false
            });
        });

        it('says when some of the turn did not arrive', () => {
            loadThinking$.mockReturnValue(throwError(() => new Error('offline')));
            renderLiveHanded();

            emit([thought('third')], 3);

            expect(handed.at(-1)).toEqual({ idTask: 9, events: [thought('third')], hasGap: true });
        });

        it('hands nothing before the first event', () => {
            renderLiveHanded();

            expect(handed).toEqual([]);
        });
    });
});

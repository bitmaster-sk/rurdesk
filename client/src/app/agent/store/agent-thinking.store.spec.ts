import { DestroyRef, Injector, runInInjectionContext } from '@angular/core';
import { Subject } from 'rxjs';
import { AgentThinkingStore } from './agent-thinking.store';
import { THINKING_BUFFER_CHARS } from '../constants/agent-thinking.constants';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { AgentThinkingKind } from '../constants/agent-thinking-kind.enum';
import { AgentToolKind } from '../constants/agent-tool-kind.enum';
import { AgentThinkingNotice } from '../model/agent-thinking.model';

function makeNotice(
    idRun: number,
    seq: number,
    text: string,
    stage = 'implementation',
    idTask = 1
): { payload: AgentThinkingNotice } {
    return {
        payload: {
            idRun,
            idTask,
            stage,
            seq,
            events: [{ kind: AgentThinkingKind.Thinking, text, at: seq }]
        }
    };
}

function build(agentThinking$: Subject<{ payload: AgentThinkingNotice }>): AgentThinkingStore {
    const injector = Injector.create({
        providers: [
            { provide: NoticeService, useValue: { agentThinking$ } },
            { provide: DestroyRef, useValue: { onDestroy: () => () => undefined } }
        ]
    });
    return runInInjectionContext(injector, () => new AgentThinkingStore());
}

describe('AgentThinkingStore', () => {
    // The gateway resends a failed batch under its original seq, so the same
    // events reach the reader twice unless the store drops the repeat.
    it('ignores a batch it has already applied', () => {
        const notices = new Subject<{ payload: AgentThinkingNotice }>();
        const store = build(notices);
        store.bind(5);

        notices.next(makeNotice(5, 1, 'one thought'));
        notices.next(makeNotice(5, 1, 'one thought'));

        expect(store.lines().map(line => line.label)).toEqual(['one thought']);
        expect(store.hasGap()).toBe(false);
    });

    it('still applies the batch after a repeat', () => {
        const notices = new Subject<{ payload: AgentThinkingNotice }>();
        const store = build(notices);
        store.bind(5);

        notices.next(makeNotice(5, 1, 'one '));
        notices.next(makeNotice(5, 1, 'one '));
        notices.next(makeNotice(5, 2, 'two'));

        expect(store.lines().map(line => line.label)).toEqual(['one two']);
        expect(store.hasGap()).toBe(false);
    });

    // A tool call carries its name and its argument in separate fields, so
    // counting only one of them lets the buffer grow past the cap.
    it('counts both the tool name and its argument against the buffer', () => {
        const notices = new Subject<{ payload: AgentThinkingNotice }>();
        const store = build(notices);
        store.bind(5);

        const long = 'b'.repeat(THINKING_BUFFER_CHARS);
        notices.next({
            payload: {
                idRun: 5,
                idTask: 1,
                stage: 'implementation',
                seq: 1,
                events: [
                    { kind: AgentThinkingKind.Tool, tool: long, text: '', at: 1 },
                    { kind: AgentThinkingKind.Tool, tool: long, text: '', at: 2 }
                ]
            }
        });

        expect(store.events()).toHaveLength(1);
        expect(store.hasGap()).toBe(true);
    });

    it('appends the events of the bound run in arrival order', () => {
        const notices = new Subject<{ payload: AgentThinkingNotice }>();
        const store = build(notices);
        store.bind(5);

        notices.next(makeNotice(5, 1, 'first thought'));
        notices.next(makeNotice(5, 2, 'second thought'));

        expect(store.events().map(event => event.text)).toEqual([
            'first thought',
            'second thought'
        ]);
        expect(store.stage()).toBe('implementation');
    });

    it('ignores notices for another run', () => {
        const notices = new Subject<{ payload: AgentThinkingNotice }>();
        const store = build(notices);
        store.bind(5);

        notices.next(makeNotice(9, 1, 'someone else'));

        expect(store.events()).toEqual([]);
    });

    it('ignores notices before a run is bound', () => {
        const notices = new Subject<{ payload: AgentThinkingNotice }>();
        const store = build(notices);

        notices.next(makeNotice(5, 1, 'too early'));

        expect(store.events()).toEqual([]);
    });

    it('drops the oldest events once the buffer cap is reached', () => {
        const notices = new Subject<{ payload: AgentThinkingNotice }>();
        const store = build(notices);
        store.bind(5);

        notices.next(makeNotice(5, 1, 'the oldest thought'));
        notices.next(makeNotice(5, 2, 'x'.repeat(THINKING_BUFFER_CHARS)));

        const texts = store.events().map(event => event.text);
        expect(texts).not.toContain('the oldest thought');
        expect(texts.join('').length).toBeLessThanOrEqual(THINKING_BUFFER_CHARS);
    });

    it('flags the thinking it discarded to stay inside the buffer', () => {
        const notices = new Subject<{ payload: AgentThinkingNotice }>();
        const store = build(notices);
        store.bind(5);

        notices.next(makeNotice(5, 1, 'the oldest thought'));
        expect(store.hasGap()).toBe(false);

        notices.next(makeNotice(5, 2, 'x'.repeat(THINKING_BUFFER_CHARS)));

        expect(store.hasGap()).toBe(true);
    });

    it('flags a missing batch instead of pretending the stream is continuous', () => {
        const notices = new Subject<{ payload: AgentThinkingNotice }>();
        const store = build(notices);
        store.bind(5);

        notices.next(makeNotice(5, 1, 'first'));
        expect(store.hasGap()).toBe(false);

        notices.next(makeNotice(5, 3, 'third'));

        expect(store.hasGap()).toBe(true);
        expect(store.events().map(event => event.text)).toEqual(['first', 'third']);
    });

    it('flags joining a stage that is already mid-stream', () => {
        const notices = new Subject<{ payload: AgentThinkingNotice }>();
        const store = build(notices);
        store.bind(5);

        notices.next(makeNotice(5, 57, 'the reader opened the issue late'));

        expect(store.hasGap()).toBe(true);
    });

    it('starts a new stage with an empty buffer', () => {
        const notices = new Subject<{ payload: AgentThinkingNotice }>();
        const store = build(notices);
        store.bind(5);

        notices.next(makeNotice(5, 1, 'design thinking', 'design', 1));
        notices.next(makeNotice(5, 1, 'implementation thinking', 'implementation', 2));

        expect(store.events().map(event => event.text)).toEqual(['implementation thinking']);
        expect(store.stage()).toBe('implementation');
    });

    // Continue creates a second task for the same stage and the gateway restarts
    // seq at 1, so the stage name alone cannot tell the attempts apart.
    it('starts a retried stage with an empty buffer', () => {
        const notices = new Subject<{ payload: AgentThinkingNotice }>();
        const store = build(notices);
        store.bind(5);

        notices.next(makeNotice(5, 1, 'the attempt that failed', 'design', 1));
        notices.next(makeNotice(5, 1, 'the retry', 'design', 2));

        expect(store.events().map(event => event.text)).toEqual(['the retry']);
        expect(store.hasGap()).toBe(false);
    });

    it('clears everything when the bound run changes', () => {
        const notices = new Subject<{ payload: AgentThinkingNotice }>();
        const store = build(notices);
        store.bind(5);
        notices.next(makeNotice(5, 1, 'old run'));

        store.bind(6);

        expect(store.events()).toEqual([]);
        expect(store.idTask()).toBeNull();
        expect(store.stage()).toBeNull();
        expect(store.hasGap()).toBe(false);
    });

    // A reload starts the stream mid-stage, and the thinking before it is on the
    // server, not lost.
    it('replays the stored thinking of a stage joined mid-stream', () => {
        const notices = new Subject<{ payload: AgentThinkingNotice }>();
        const store = build(notices);
        store.bind(5);

        notices.next(makeNotice(5, 4, 'the batch that arrived first'));
        store.applyStored(
            'implementation',
            1,
            [{ kind: AgentThinkingKind.Thinking, text: 'what came before', at: 1 }],
            3
        );

        expect(store.events().map(event => event.text)).toEqual([
            'what came before',
            'the batch that arrived first'
        ]);
        expect(store.hasGap()).toBe(false);
    });

    it('keeps flagging a gap the stored thinking does not close', () => {
        const notices = new Subject<{ payload: AgentThinkingNotice }>();
        const store = build(notices);
        store.bind(5);

        notices.next(makeNotice(5, 9, 'the batch that arrived first'));
        store.applyStored(
            'implementation',
            1,
            [{ kind: AgentThinkingKind.Thinking, text: 'what came before', at: 1 }],
            3
        );

        expect(store.hasGap()).toBe(true);
    });

    it('replays the stored thinking before any batch arrives', () => {
        const notices = new Subject<{ payload: AgentThinkingNotice }>();
        const store = build(notices);
        store.bind(5);

        store.applyStored(
            'implementation',
            1,
            [{ kind: AgentThinkingKind.Thinking, text: 'what came before', at: 1 }],
            3
        );
        notices.next(makeNotice(5, 4, 'the next batch'));

        expect(store.events().map(event => event.text)).toEqual([
            'what came before',
            'the next batch'
        ]);
        expect(store.hasGap()).toBe(false);
    });

    // The row that fetches is recreated with the feed, the store outlives it, so
    // the same stored thinking can arrive twice.
    it('ignores stored thinking it has already applied', () => {
        const notices = new Subject<{ payload: AgentThinkingNotice }>();
        const store = build(notices);
        store.bind(5);
        const stored = [{ kind: AgentThinkingKind.Thinking, text: 'what came before', at: 1 }];

        store.applyStored('implementation', 1, stored, 3);
        store.applyStored('implementation', 1, stored, 3);

        expect(store.events().map(event => event.text)).toEqual(['what came before']);
    });

    // The fetch and the stream race, so the reply can describe an attempt the
    // stream has already moved past.
    it('ignores stored thinking of another stage or attempt', () => {
        const notices = new Subject<{ payload: AgentThinkingNotice }>();
        const store = build(notices);
        store.bind(5);

        notices.next(makeNotice(5, 1, 'the retry', 'design', 2));
        store.applyStored(
            'design',
            1,
            [{ kind: AgentThinkingKind.Thinking, text: 'the attempt that failed', at: 1 }],
            3
        );

        expect(store.events().map(event => event.text)).toEqual(['the retry']);
    });

    it('replays a batch the stream delivers again after the replay', () => {
        const notices = new Subject<{ payload: AgentThinkingNotice }>();
        const store = build(notices);
        store.bind(5);

        store.applyStored(
            'implementation',
            1,
            [{ kind: AgentThinkingKind.Thinking, text: 'what came before', at: 1 }],
            3
        );
        notices.next(makeNotice(5, 3, 'the batch already stored'));

        expect(store.events().map(event => event.text)).toEqual(['what came before']);
    });

    it('joins fragments of one thought instead of breaking them into lines', () => {
        const notices = new Subject<{ payload: AgentThinkingNotice }>();
        const store = build(notices);
        store.bind(5);

        notices.next(makeNotice(5, 1, 'does it return an object'));
        notices.next(makeNotice(5, 2, '? Let us investigate.'));

        expect(store.lines()).toEqual([
            {
                kind: AgentThinkingKind.Thinking,
                label: 'does it return an object? Let us investigate.',
                detail: '',
                toolKind: AgentToolKind.Other
            }
        ]);
    });

    it('keeps the tool call and its argument together as one step', () => {
        const notices = new Subject<{ payload: AgentThinkingNotice }>();
        const store = build(notices);
        store.bind(5);

        notices.next({
            payload: {
                idRun: 5,
                idTask: 1,
                stage: 'implementation',
                seq: 1,
                events: [
                    { kind: AgentThinkingKind.Thinking, text: 'let me look', at: 1 },
                    {
                        kind: AgentThinkingKind.Tool,
                        tool: 'developer__shell',
                        text: 'rg --files src',
                        at: 2
                    },
                    { kind: AgentThinkingKind.Thinking, text: 'that listing helps', at: 3 }
                ]
            }
        });

        expect(store.lines()).toEqual([
            {
                kind: AgentThinkingKind.Thinking,
                label: 'let me look',
                detail: '',
                toolKind: AgentToolKind.Other
            },
            {
                kind: AgentThinkingKind.Tool,
                label: 'developer__shell',
                detail: 'rg --files src',
                toolKind: AgentToolKind.Run
            },
            {
                kind: AgentThinkingKind.Thinking,
                label: 'that listing helps',
                detail: '',
                toolKind: AgentToolKind.Other
            }
        ]);
    });
});

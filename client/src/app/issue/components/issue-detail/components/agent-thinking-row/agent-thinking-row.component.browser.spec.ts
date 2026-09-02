import { TestBed } from '@angular/core/testing';
import { Subject, of, throwError } from 'rxjs';
import { TranslateModule } from '@ngx-translate/core';
import { AvatarStub, TablerIconStub } from 'src/testing/stubs';
import { AgentThinkingApi } from 'src/app/agent/api/agent-thinking.api.service';
import { AgentStageProgress } from 'src/app/agent/model/agent-run.model';
import { AgentThinkingKind } from 'src/app/agent/constants/agent-thinking-kind.enum';
import { AgentThinkingNotice } from 'src/app/agent/model/agent-thinking.model';
import { AgentThinkingStore } from 'src/app/agent/store/agent-thinking.store';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { AgentThinkingRowComponent } from './agent-thinking-row.component';

describe('AgentThinkingRowComponent (browser)', () => {
    let loadStageThinking$: ReturnType<typeof vi.fn>;
    let notices: Subject<{ payload: AgentThinkingNotice }>;
    let store: AgentThinkingStore;

    beforeEach(async () => {
        notices = new Subject();
        loadStageThinking$ = vi.fn().mockReturnValue(
            of({
                idRun: 5,
                stage: 'design',
                events: [{ kind: AgentThinkingKind.Thinking, text: 'the full thinking', at: 1 }],
                isComplete: true
            })
        );
        await TestBed.configureTestingModule({
            declarations: [AgentThinkingRowComponent],
            imports: [TranslateModule.forRoot(), TablerIconStub, AvatarStub],
            providers: [
                AgentThinkingStore,
                { provide: NoticeService, useValue: { agentThinking$: notices } },
                { provide: AgentThinkingApi, useValue: { loadStageThinking$ } }
            ]
        }).compileComponents();
        store = TestBed.inject(AgentThinkingStore);
    });

    function render(stage: AgentStageProgress, isLive = false) {
        const fixture = TestBed.createComponent(AgentThinkingRowComponent);
        fixture.componentRef.setInput('idRun', 5);
        fixture.componentRef.setInput('stage', stage);
        fixture.componentRef.setInput('isLive', isLive);
        fixture.detectChanges();
        return fixture;
    }

    function renderLive(width?: string) {
        store.bind(5);
        const fixture = render({ stage: 'implementation', status: 'active' }, true);
        if (width) {
            const host = fixture.nativeElement as HTMLElement;
            host.style.display = 'block';
            host.style.width = width;
        }
        return fixture;
    }

    function emit(events: AgentThinkingNotice['events'], seq = 1): void {
        notices.next({
            payload: { idRun: 5, idTask: 1, stage: 'implementation', seq, events }
        });
    }

    function toggle(fixture: ReturnType<typeof render>): void {
        fixture.nativeElement.querySelector('.thinking-head').click();
        fixture.detectChanges();
    }

    function body(fixture: ReturnType<typeof render>): HTMLElement {
        return fixture.nativeElement.querySelector('[data-testid="agent-thinking-body"]');
    }

    it('renders a finished stage collapsed, without fetching anything', () => {
        const fixture = render({ stage: 'design', status: 'done', hasThinking: true });

        expect(body(fixture)).toBeNull();
        expect(loadStageThinking$).not.toHaveBeenCalled();
    });

    it('fetches the stored thinking once on expand', () => {
        const fixture = render({ stage: 'design', status: 'done', hasThinking: true });

        toggle(fixture);
        expect(body(fixture).textContent).toContain('the full thinking');

        toggle(fixture);
        toggle(fixture);
        expect(loadStageThinking$).toHaveBeenCalledTimes(1);
        expect(loadStageThinking$).toHaveBeenCalledWith(5, 'design');
    });

    // A finished stage must show the same name and argument the live stream
    // showed, not a bare tool name.
    it('shows the argument of a stored tool call', () => {
        loadStageThinking$.mockReturnValue(
            of({
                idRun: 5,
                stage: 'design',
                events: [
                    { kind: AgentThinkingKind.Thinking, text: 'weighing options', at: 1 },
                    {
                        kind: AgentThinkingKind.Tool,
                        tool: 'shell',
                        text: 'rg --files src',
                        at: 2
                    },
                    { kind: AgentThinkingKind.Thinking, text: 'now writing', at: 3 }
                ],
                isComplete: true
            })
        );
        const fixture = render({ stage: 'design', status: 'done', hasThinking: true });

        toggle(fixture);
        const act = body(fixture).querySelector('.thinking-act') as HTMLElement;
        expect(act.querySelector('.thinking-act-name')!.textContent).toContain('shell');
        expect(act.querySelector('.thinking-act-arg')!.textContent).toContain('rg --files src');
        expect(act.getAttribute('data-kind')).toBe('run');
    });

    // Thinking that reads like a marker is prose the model wrote. Nothing parses
    // the stored form, so it can never be promoted into a tool step.
    it('renders arrow-prefixed stored thinking as a thought, not a tool call', () => {
        loadStageThinking$.mockReturnValue(
            of({
                idRun: 5,
                stage: 'design',
                events: [
                    {
                        kind: AgentThinkingKind.Thinking,
                        text: '→ shell returns nil when the file is missing',
                        at: 1
                    }
                ],
                isComplete: true
            })
        );
        const fixture = render({ stage: 'design', status: 'done', hasThinking: true });

        toggle(fixture);
        expect(body(fixture).querySelector('.thinking-act')).toBeNull();
        expect(body(fixture).querySelector('.thinking-thought')!.textContent).toContain(
            '→ shell returns nil when the file is missing'
        );
    });

    // The marker is the API's own, so the reader is told the tail is missing
    // rather than seeing the stage simply stop.
    it('names the per-stage limit when a stage was truncated', () => {
        loadStageThinking$.mockReturnValue(
            of({
                idRun: 5,
                stage: 'design',
                events: [
                    { kind: AgentThinkingKind.Thinking, text: 'a long thought', at: 1 },
                    { kind: AgentThinkingKind.Truncated, at: 2 }
                ],
                isComplete: true
            })
        );
        const fixture = render({ stage: 'design', status: 'done', hasThinking: true });

        toggle(fixture);
        expect(
            body(fixture).querySelector('[data-testid="agent-thinking-truncated"]')
        ).not.toBeNull();
    });

    // The chip is picked from four kinds, never from the tool name, so a tool
    // nobody has mapped still renders a chip instead of an empty box.
    it('gives an unmapped tool the neutral chip', () => {
        const fixture = renderLive();

        emit([
            { kind: AgentThinkingKind.Tool, tool: 'quantum__entangle', text: 'qubits', at: 1 },
            { kind: AgentThinkingKind.Tool, tool: 'tracker__get_issue', text: '{}', at: 2 }
        ]);
        fixture.detectChanges();

        const acts = [...body(fixture).querySelectorAll('.thinking-act')];
        expect(acts.map(act => act.getAttribute('data-kind'))).toEqual(['other', 'read']);
        acts.forEach(act =>
            expect(act.querySelector('.thinking-act-chip tabler-icon')).not.toBeNull()
        );
    });

    // Marking the fetch done before it lands would strand the row empty for the
    // life of the component: no collapse and re-expand would ever try again.
    it('retries the fetch on the next expand after it failed', () => {
        loadStageThinking$.mockReturnValueOnce(throwError(() => new Error('offline')));
        const fixture = render({ stage: 'design', status: 'done', hasThinking: true });

        toggle(fixture);
        expect(body(fixture).textContent).not.toContain('the full thinking');

        toggle(fixture);
        toggle(fixture);
        expect(loadStageThinking$).toHaveBeenCalledTimes(2);
        expect(body(fixture).textContent).toContain('the full thinking');
    });

    // A failed stage is compacted tail-only, and the hourly job can fold its
    // leftover rows into the blob afterwards. The row keeps its instance across
    // that, so the tail must not stand in for thinking that now exists.
    it('fetches the full thinking once it replaces the tail', () => {
        const fixture = render({
            stage: 'design',
            status: 'done',
            hasThinking: false,
            thinkingTail: 'only the tail survived'
        });

        toggle(fixture);
        expect(body(fixture).textContent).toContain('only the tail survived');
        expect(loadStageThinking$).not.toHaveBeenCalled();

        toggle(fixture);
        fixture.componentRef.setInput('stage', {
            stage: 'design',
            status: 'done',
            hasThinking: true
        });
        toggle(fixture);

        expect(loadStageThinking$).toHaveBeenCalledTimes(1);
        expect(body(fixture).textContent).toContain('the full thinking');
    });

    it('shows the tail with a badge and no fetch when full text was not kept', () => {
        const fixture = render({
            stage: 'design',
            status: 'done',
            hasThinking: false,
            thinkingTail: 'only the tail survived'
        });

        expect(
            fixture.nativeElement.querySelector('[data-testid="agent-thinking-tail-badge"]')
        ).not.toBeNull();

        toggle(fixture);
        expect(body(fixture).textContent).toContain('only the tail survived');
        expect(loadStageThinking$).not.toHaveBeenCalled();
    });

    // A reload leaves the reader mid-stage, and the stream only carries what
    // comes next, so the row asks for what the stage has thought so far.
    it('replays the stored thinking of a running stage it joined mid-stream', () => {
        loadStageThinking$.mockReturnValue(
            of({
                idRun: 5,
                idTask: 1,
                stage: 'implementation',
                events: [{ kind: AgentThinkingKind.Thinking, text: 'what came before', at: 1 }],
                lastSeq: 3,
                isComplete: false
            })
        );
        const fixture = renderLive();

        expect(loadStageThinking$).toHaveBeenCalledWith(5, 'implementation');
        emit([{ kind: AgentThinkingKind.Thinking, text: ' and what came after', at: 2 }], 4);
        fixture.detectChanges();

        expect(body(fixture).textContent).toContain('what came before and what came after');
        expect(fixture.nativeElement.querySelector('.thinking-gap')).toBeNull();
    });

    it('opens a running stage by default and appends what streams in', () => {
        const fixture = renderLive();

        expect(
            fixture.nativeElement.querySelector('[data-testid="agent-thinking-working"]')
        ).not.toBeNull();
        expect(loadStageThinking$).toHaveBeenCalledTimes(1);

        emit([{ kind: AgentThinkingKind.Thinking, text: 'weighing the tokenizer', at: 1 }]);
        emit([{ kind: AgentThinkingKind.Thinking, text: ' and the tests', at: 2 }], 2);
        fixture.detectChanges();

        expect(body(fixture).textContent).toContain('weighing the tokenizer and the tests');
    });

    // The live row is reused across a stage switch, so without a guard the next
    // stage opens showing the previous stage's thinking as its own.
    it('drops the previous stage thinking when the row moves to the next stage', () => {
        const fixture = renderLive();

        emit([{ kind: AgentThinkingKind.Thinking, text: 'brainstorming the options', at: 1 }]);
        fixture.detectChanges();
        expect(body(fixture).textContent).toContain('brainstorming the options');

        fixture.componentRef.setInput('stage', { stage: 'design', status: 'active' });
        fixture.detectChanges();

        expect(body(fixture).textContent).not.toContain('brainstorming the options');
    });

    // Between the stage starting and the run snapshot naming it, the row has no
    // stage of its own — it must still show what is streaming.
    it('shows the stream while the new stage is still unnamed', () => {
        store.bind(5);
        const fixture = render({ stage: '', status: 'active' }, true);

        emit([{ kind: AgentThinkingKind.Thinking, text: 'reading the plan', at: 1 }]);
        fixture.detectChanges();

        expect(body(fixture).textContent).toContain('reading the plan');
    });

    it('lets the reader collapse a running stage', () => {
        const fixture = renderLive();

        toggle(fixture);
        expect(body(fixture)).toBeNull();
    });

    // A long shell command must never spill past the card: the whole right-hand
    // column starts scrolling sideways while the agent works.
    it('keeps a long tool argument inside the card', () => {
        const fixture = renderLive('400px');

        emit([
            {
                kind: AgentThinkingKind.Tool,
                tool: 'developer__shell',
                text: 'rg --files --hidden --glob !node_modules ' + 'x'.repeat(300),
                at: 1
            }
        ]);
        fixture.detectChanges();

        const arg = fixture.nativeElement.querySelector('.thinking-act-arg') as HTMLElement;
        expect(arg.scrollWidth).toBeLessThanOrEqual(arg.clientWidth);
    });

    it('wraps a long thinking line inside the card', () => {
        const fixture = renderLive('400px');

        emit([{ kind: AgentThinkingKind.Thinking, text: 'y'.repeat(500), at: 1 }]);
        fixture.detectChanges();

        const line = fixture.nativeElement.querySelector('.thinking-thought') as HTMLElement;
        expect(line.scrollWidth).toBeLessThanOrEqual(line.clientWidth);
    });

    // goose renders the agent's thinking as markdown, so the newlines the model
    // wrote carry its structure — lists, numbered steps — and must survive.
    it('preserves the newlines the agent wrote', () => {
        const fixture = renderLive();

        emit([
            {
                kind: AgentThinkingKind.Thinking,
                text: 'Files to create:\n- logic.ts\n- index.ts',
                at: 1
            }
        ]);
        fixture.detectChanges();

        const line = fixture.nativeElement.querySelector('.thinking-thought') as HTMLElement;
        expect(getComputedStyle(line).whiteSpace).toBe('pre-wrap');
        expect(body(fixture).textContent).toContain('- logic.ts');
    });
});

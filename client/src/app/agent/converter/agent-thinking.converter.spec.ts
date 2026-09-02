import { AgentThinkingKind } from '../constants/agent-thinking-kind.enum';
import { AgentToolKind } from '../constants/agent-tool-kind.enum';
import { AgentThinkingEvent } from '../model/agent-thinking.model';
import { AgentThinkingConverter } from './agent-thinking.converter';

describe('AgentThinkingConverter', () => {
    function thinking(text: string): AgentThinkingEvent {
        return { kind: AgentThinkingKind.Thinking, text, at: 1 };
    }

    function tool(name: string, detail = ''): AgentThinkingEvent {
        return { kind: AgentThinkingKind.Tool, tool: name, text: detail, at: 1 };
    }

    it('returns nothing for an empty stream', () => {
        expect(AgentThinkingConverter.toLines([])).toEqual([]);
    });

    it('keeps a tool call name and argument apart', () => {
        const lines = AgentThinkingConverter.toLines([tool('shell', 'rg --files src')]);

        expect(lines).toEqual([
            {
                kind: AgentThinkingKind.Tool,
                label: 'shell',
                detail: 'rg --files src',
                toolKind: AgentToolKind.Run
            }
        ]);
    });

    it('keeps a tool call without an argument', () => {
        const lines = AgentThinkingConverter.toLines([tool('tracker__complete_stage')]);

        expect(lines).toEqual([
            {
                kind: AgentThinkingKind.Tool,
                label: 'tracker__complete_stage',
                detail: '',
                toolKind: AgentToolKind.Write
            }
        ]);
    });

    // Consecutive fragments are one sentence cut at a flush boundary.
    it('joins consecutive thinking into one line', () => {
        const lines = AgentThinkingConverter.toLines([
            thinking('does it return an object'),
            thinking('? Let us investigate.'),
            tool('shell', 'ls'),
            thinking('the listing helps.')
        ]);

        expect(lines.map(line => line.label)).toEqual([
            'does it return an object? Let us investigate.',
            'shell',
            'the listing helps.'
        ]);
    });

    // The model writes arrows and brackets in ordinary prose. Nothing in the
    // pipeline parses that text, so none of it can turn into a tool step.
    it.each(['→ shell returns nil', '[#tool#] is a literal here', '-> read the plan'])(
        'renders %s as thinking',
        text => {
            const lines = AgentThinkingConverter.toLines([thinking(text)]);

            expect(lines).toEqual([
                {
                    kind: AgentThinkingKind.Thinking,
                    label: text,
                    detail: '',
                    toolKind: AgentToolKind.Other
                }
            ]);
        }
    );

    // A multi-line tool argument stays one step; there is no line-based split
    // that could break it into a fake thought.
    it('keeps a multi-line tool argument on its own step', () => {
        const lines = AgentThinkingConverter.toLines([tool('shell', 'git commit -m "one\ntwo"')]);

        expect(lines).toHaveLength(1);
        expect(lines[0].detail).toBe('git commit -m "one\ntwo"');
    });

    it('carries the truncation marker through as its own line', () => {
        const lines = AgentThinkingConverter.toLines([
            thinking('a thought'),
            { kind: AgentThinkingKind.Truncated, at: 2 }
        ]);

        expect(lines.map(line => line.kind)).toEqual([
            AgentThinkingKind.Thinking,
            AgentThinkingKind.Truncated
        ]);
    });

    describe('toToolKind', () => {
        // Every name the agent actually called, taken from the gateway log of a
        // real run. An unmapped name is not a defect — it falls to Other.
        it.each([
            ['shell', AgentToolKind.Run],
            ['todo__todo_write', AgentToolKind.Write],
            ['analyze', AgentToolKind.Read],
            ['tracker__complete_stage', AgentToolKind.Write],
            ['edit', AgentToolKind.Write],
            ['write', AgentToolKind.Write],
            ['tree', AgentToolKind.Read],
            ['tracker__get_issue', AgentToolKind.Read],
            ['tracker__get_project_context', AgentToolKind.Read],
            ['tracker__list_relations', AgentToolKind.Read],
            ['read', AgentToolKind.Read]
        ])('maps %s', (name, kind) => {
            expect(AgentThinkingConverter.toToolKind(name)).toBe(kind);
        });

        it('falls back to Other for a tool it has never seen', () => {
            expect(AgentThinkingConverter.toToolKind('quantum__entangle')).toBe(
                AgentToolKind.Other
            );
        });

        it('is not confused by case', () => {
            expect(AgentThinkingConverter.toToolKind('Developer__Shell')).toBe(AgentToolKind.Run);
        });
    });
});

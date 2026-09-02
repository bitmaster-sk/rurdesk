import { AgentThinkingKind } from '../constants/agent-thinking-kind.enum';
import { AgentToolKind } from '../constants/agent-tool-kind.enum';
import { AgentThinkingLine } from '../entity/agent-thinking.entity';
import { AgentThinkingEvent } from '../model/agent-thinking.model';

export abstract class AgentThinkingConverter {
    private static readonly toolKindRules: [AgentToolKind, string[]][] = [
        [AgentToolKind.Run, ['shell']],
        [AgentToolKind.Write, ['write', 'edit', 'create', 'update', 'post', 'complete']],
        [
            AgentToolKind.Read,
            ['read', 'get', 'list', 'search', 'find', 'grep', 'tree', 'analyze', 'context']
        ]
    ];

    public static toLines(events: AgentThinkingEvent[]): AgentThinkingLine[] {
        const lines: AgentThinkingLine[] = [];
        for (const event of events) {
            if (event.kind === AgentThinkingKind.Thinking) {
                const previous = lines[lines.length - 1];
                // Consecutive fragments are one sentence cut at a flush boundary.
                if (previous?.kind === AgentThinkingKind.Thinking) {
                    previous.label += event.text ?? '';
                    continue;
                }
                lines.push(this.toLine(event.kind, event.text ?? '', '', AgentToolKind.Other));
                continue;
            }
            if (event.kind === AgentThinkingKind.Tool) {
                const tool = event.tool ?? '';
                lines.push(this.toLine(event.kind, tool, event.text ?? '', this.toToolKind(tool)));
                continue;
            }
            lines.push(this.toLine(event.kind, '', '', AgentToolKind.Other));
        }
        return lines;
    }

    public static toTailEvents(tail: string): AgentThinkingEvent[] {
        return tail === '' ? [] : [{ kind: AgentThinkingKind.Thinking, text: tail, at: 0 }];
    }

    public static toToolKind(name: string): AgentToolKind {
        const lower = name.toLowerCase();
        for (const [kind, needles] of this.toolKindRules) {
            if (needles.some(needle => lower.includes(needle))) {
                return kind;
            }
        }
        return AgentToolKind.Other;
    }

    private static toLine(
        kind: AgentThinkingKind,
        label: string,
        detail: string,
        toolKind: AgentToolKind
    ): AgentThinkingLine {
        return { kind, label, detail, toolKind };
    }
}

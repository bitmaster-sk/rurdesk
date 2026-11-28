import { describe, expect, it } from 'vitest';
import { AgentConversationListItem } from '../model/agent-conversation.model';
import { ConversationGrouper } from './conversation-grouper';

function conversation(
    idConversation: number,
    idProject: number | null,
    pinnedAt: string | null = null
): AgentConversationListItem {
    return { idConversation, idProject, pinnedAt } as unknown as AgentConversationListItem;
}

function ids(sections: ReturnType<typeof ConversationGrouper.group>): unknown[] {
    return sections.map(section => [
        section.titleKey ?? section.title,
        section.items.map(item => item.idConversation)
    ]);
}

describe('ConversationGrouper', () => {
    const list = [
        conversation(1, 5),
        conversation(2, 7),
        conversation(3, 7, '2026-11-01'),
        conversation(4, null),
        conversation(5, 5, '2026-11-02'),
        conversation(6, 5)
    ];

    it('puts the pinned first, then the open project, then the other projects', () => {
        expect(ids(ConversationGrouper.group(list, { idProject: 5, name: 'Apollo' }))).toEqual([
            ['AGENT_CHAT.PINNED', [3, 5]],
            ['Apollo', [1, 6]],
            ['AGENT_CHAT.OTHER_PROJECTS', [2, 4]]
        ]);
    });

    it('has only the pinned and the recent without a project', () => {
        expect(ids(ConversationGrouper.group(list, null))).toEqual([
            ['AGENT_CHAT.PINNED', [3, 5]],
            ['AGENT_CHAT.RECENT', [1, 2, 4, 6]]
        ]);
    });

    it('drops the sections left empty', () => {
        const unpinned = [conversation(1, 7), conversation(2, 7)];

        expect(ids(ConversationGrouper.group(unpinned, { idProject: 5, name: 'Apollo' }))).toEqual([
            ['AGENT_CHAT.OTHER_PROJECTS', [1, 2]]
        ]);
        expect(ids(ConversationGrouper.group([], null))).toEqual([]);
    });
});

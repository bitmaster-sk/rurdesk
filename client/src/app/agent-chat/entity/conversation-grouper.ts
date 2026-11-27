import { AgentConversationListItem } from '../model/agent-conversation.model';

/** A labelled group of the conversation list: a translation key or a project name. */
export interface ConversationSection {
    titleKey?: string;
    title?: string;
    items: AgentConversationListItem[];
}

export abstract class ConversationGrouper {
    /**
     * Pinned first, then the open project's conversations under its name, then the other projects;
     * without a project, the pinned and the recent. Each section keeps the server's order and an
     * empty one is dropped.
     */
    public static group(
        list: AgentConversationListItem[],
        project: { idProject: number; name: string } | null
    ): ConversationSection[] {
        const pinned = list.filter(c => c.pinnedAt !== null);
        const unpinned = list.filter(c => c.pinnedAt === null);
        const sections: ConversationSection[] = [{ titleKey: 'AGENT_CHAT.PINNED', items: pinned }];
        if (project === null) {
            sections.push({ titleKey: 'AGENT_CHAT.RECENT', items: unpinned });
        } else {
            sections.push(
                {
                    title: project.name,
                    items: unpinned.filter(c => c.idProject === project.idProject)
                },
                {
                    titleKey: 'AGENT_CHAT.OTHER_PROJECTS',
                    items: unpinned.filter(c => c.idProject !== project.idProject)
                }
            );
        }
        return sections.filter(section => section.items.length > 0);
    }
}

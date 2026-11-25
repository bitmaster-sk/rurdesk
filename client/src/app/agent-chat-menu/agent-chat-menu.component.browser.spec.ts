import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';
import { Subject, of } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { AgentConversationApi } from 'src/app/agent-chat/api/agent-conversation.api.service';
import { AgentConversationListItem } from 'src/app/agent-chat/model/agent-conversation.model';
import { AgentConversationUnreadStore } from 'src/app/agent-chat/store/agent-conversation-unread.store';
import { ProjectContextStore } from 'src/app/home/store/project-context.store';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { UiMenuItem } from 'src/app/ui/components/menu/menu-item.model';
import { AgentChatMenuComponent } from './agent-chat-menu.component';

function conversation(
    idConversation: number,
    title: string,
    idProject: number | null,
    pinnedAt: string | null = null
): AgentConversationListItem {
    const item: Partial<AgentConversationListItem> = {
        idConversation,
        title,
        idProject,
        pinnedAt,
        isUnread: false
    };
    return item as AgentConversationListItem;
}

interface Internals {
    items(): UiMenuItem[];
    badgeText(): string;
}

async function setup(
    conversations: AgentConversationListItem[],
    count = 0,
    idProject: number | null = null,
    idContextProject: number | null = null
): Promise<{
    menu: Internals;
    navigate: ReturnType<typeof vi.fn>;
    fixture: unknown;
    load$: ReturnType<typeof vi.fn>;
    changed$: Subject<void>;
}> {
    const navigate = vi.fn();
    const load$ = vi.fn(() => of(conversations));
    const changed$ = new Subject<void>();
    await TestBed.configureTestingModule({
        declarations: [AgentChatMenuComponent],
        providers: [
            { provide: AgentConversationApi, useValue: { load$ } },
            {
                provide: AgentConversationUnreadStore,
                useValue: { count: signal(count), load: () => {}, changed$ }
            },
            { provide: NoticeService, useValue: { agentConversation$: new Subject() } },
            { provide: Router, useValue: { navigate } },
            { provide: ProjectContextStore, useValue: { idProject: signal(idContextProject) } }
        ]
    })
        .overrideComponent(AgentChatMenuComponent, { set: { template: '' } })
        .compileComponents();

    const fixture = TestBed.createComponent(AgentChatMenuComponent);
    fixture.componentRef.setInput('idProject', idProject);
    fixture.detectChanges();
    return {
        menu: fixture.componentInstance as unknown as Internals,
        navigate,
        fixture,
        load$,
        changed$
    };
}

const labels = (items: UiMenuItem[]): string[] =>
    items.map(i => (i.separator ? '---' : (i.labelKey ?? i.label ?? '')));

describe('AgentChatMenuComponent (browser)', () => {
    it('lists current-project conversations first, then others, then the actions', async () => {
        const { menu } = await setup(
            [
                conversation(1, 'Other A', 2),
                conversation(2, 'Mine A', 1),
                conversation(3, 'No project', null),
                conversation(4, 'Mine B', 1)
            ],
            0,
            1
        );
        expect(labels(menu.items())).toEqual([
            'Mine A',
            'Mine B',
            'Other A',
            'No project',
            '---',
            'AGENT_CHAT.NEW',
            'AGENT_CHAT.ALL'
        ]);
    });

    it('takes the side menu project when the top menu has none selected', async () => {
        const { menu } = await setup(
            [conversation(1, 'Other', 2), conversation(2, 'Context', 7)],
            0,
            null,
            7
        );
        expect(labels(menu.items()).slice(0, 2)).toEqual(['Context', 'Other']);
    });

    it('lists pinned conversations first with a pin icon, then the rest in the usual order', async () => {
        const { menu } = await setup(
            [
                conversation(1, 'Pinned elsewhere', 2, '2026-11-16T11:00:00Z'),
                conversation(2, 'Pinned mine', 1, '2026-11-16T10:00:00Z'),
                conversation(3, 'Mine', 1),
                conversation(4, 'Other', 2)
            ],
            0,
            1
        );
        const recent = menu.items().filter(i => i.routerLink);

        expect(recent.map(i => i.label)).toEqual([
            'Pinned elsewhere',
            'Pinned mine',
            'Mine',
            'Other'
        ]);
        expect(recent.map(i => i.icon)).toEqual(['pin', 'pin', 'robot', 'robot']);
    });

    it('keeps a pinned conversation within the 5 shown even when it is old', async () => {
        const many = Array.from({ length: 6 }, (_, i) => conversation(i + 1, `T${i + 1}`, 1));
        many.push(conversation(7, 'Old pinned', 2, '2026-11-16T11:00:00Z'));
        const { menu } = await setup(many, 0, 1);

        const recent = menu.items().filter(i => i.routerLink);
        expect(recent).toHaveLength(5);
        expect(recent[0].label).toBe('Old pinned');
    });

    it('shows at most 5 conversations', async () => {
        const many = Array.from({ length: 8 }, (_, i) => conversation(i + 1, `T${i + 1}`, 1));
        const { menu } = await setup(many, 0, 1);
        expect(menu.items().filter(i => i.routerLink)).toHaveLength(5);
    });

    it('suffixes duplicate titles with the id so labels stay unique', async () => {
        const { menu } = await setup([
            conversation(10, 'Same', 1),
            conversation(11, 'Same', 1),
            conversation(12, 'Other', 1)
        ]);
        expect(labels(menu.items()).slice(0, 3)).toEqual(['Same #10', 'Same #11', 'Other']);
    });

    it('falls back to the id for an untitled conversation', async () => {
        const { menu } = await setup([conversation(5, '', 1)]);
        expect(labels(menu.items())[0]).toBe('#5');
    });

    it('opens a conversation through its route', async () => {
        const { menu } = await setup([conversation(5, 'A', 1)]);
        expect(menu.items()[0].routerLink).toEqual(['/agent-chat', 5]);
    });

    it('offers only the two actions, without a separator, when there are no conversations', async () => {
        const { menu } = await setup([]);
        expect(labels(menu.items())).toEqual(['AGENT_CHAT.NEW', 'AGENT_CHAT.ALL']);
    });

    it('opens the list with the project filter only for the New action scope', async () => {
        const { menu, navigate } = await setup([], 0, 3);
        const [create, all] = menu.items().filter(i => !i.separator);
        create.command?.();
        all.command?.();
        expect(navigate).toHaveBeenNthCalledWith(1, ['/agent-chat'], {
            queryParams: { idProject: 3, new: 1 }
        });
        expect(navigate).toHaveBeenNthCalledWith(2, ['/agent-chat']);
    });

    // A conversation created or read in this tab sends no notice back to it.
    it('reloads the list when this tab changed a conversation', async () => {
        const { load$, changed$ } = await setup([]);
        expect(load$).toHaveBeenCalledTimes(1);

        changed$.next();

        expect(load$).toHaveBeenCalledTimes(2);
    });

    it('shows no badge at 0 and caps it at 99+', async () => {
        expect((await setup([], 0)).menu.badgeText()).toBe('');
        TestBed.resetTestingModule();
        expect((await setup([], 7)).menu.badgeText()).toBe('7');
        TestBed.resetTestingModule();
        expect((await setup([], 100)).menu.badgeText()).toBe('99+');
    });
});

import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { RouterModule, provideRouter } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { IconPin, IconPinned, IconRobot, provideTablerIcons } from '@tabler/icons-angular';
import { afterEach, describe, expect, it } from 'vitest';
import { userEvent } from 'vitest/browser';
import { UiModule } from 'src/app/ui/ui.module';
import { TablerIconStub } from 'src/testing/stubs';
import { AgentConversationListItem } from '../../model/agent-conversation.model';
import { ConversationListComponent } from './conversation-list.component';
import 'src/styles.scss';

function item(idConversation: number, title: string): AgentConversationListItem {
    return {
        idConversation,
        title,
        idProject: 7,
        pinnedAt: null,
        isUnread: false,
        updatedAt: '2026-11-03T10:00:00Z'
    } as unknown as AgentConversationListItem;
}

@Component({
    standalone: false,
    template: `
        <div style="display: flex; height: 600px">
            <app-conversation-list
                [conversations]="conversations()"
                [idProject]="7"
                projectName="rurdesk"
                (pinChange)="pins.push($event)"
                (deleteConversation)="deletes.push($event)"
            />
        </div>
    `
})
class HostComponent {
    public readonly conversations = signal<AgentConversationListItem[]>([
        { ...item(1, 'Release notes'), pinnedAt: '2026-11-01T10:00:00Z' },
        item(2, 'Flaky e2e test'),
        { ...item(3, 'Gateway restarts'), idProject: 9, isUnread: true }
    ]);
    public readonly pins: { idConversation: number; isPinned: boolean }[] = [];
    public readonly deletes: number[] = [];
}

describe('ConversationListComponent', () => {
    let fixture: ComponentFixture<HostComponent>;

    function render(): HTMLElement {
        TestBed.configureTestingModule({
            declarations: [HostComponent, ConversationListComponent],
            imports: [UiModule, RouterModule, TablerIconStub, TranslateModule.forRoot()],
            providers: [provideRouter([]), provideTablerIcons({ IconPin, IconPinned, IconRobot })]
        });
        fixture = TestBed.createComponent(HostComponent);
        fixture.detectChanges();
        return fixture.nativeElement as HTMLElement;
    }

    function titles(host: HTMLElement): string[] {
        return Array.from(host.querySelectorAll<HTMLElement>('[data-testid="chat-list-item"]')).map(
            row => row.title
        );
    }

    function filter(host: HTMLElement, text: string): void {
        const input = host.querySelector<HTMLInputElement>('[data-testid="chat-list-filter"]')!;
        input.value = text;
        input.dispatchEvent(new Event('input'));
        fixture.detectChanges();
    }

    function menuItem(labelKey: string): HTMLElement {
        return Array.from(document.querySelectorAll<HTMLElement>('[role="menuitem"]')).find(row =>
            row.textContent.includes(labelKey)
        )!;
    }

    afterEach(() => userEvent.unhover(document.body));

    it('groups the pinned, the open project and the other projects', () => {
        const host = render();

        expect(
            Array.from(host.querySelectorAll('.ui-side-panel-section-title')).map(title =>
                title.textContent.trim()
            )
        ).toEqual(['AGENT_CHAT.PINNED', 'rurdesk', 'AGENT_CHAT.OTHER_PROJECTS']);
        expect(titles(host)).toEqual(['Release notes', 'Flaky e2e test', 'Gateway restarts']);
        expect(host.querySelector('[data-testid="chat-list-item-unread"]')!.textContent).toContain(
            'AGENT_CHAT.UNREAD'
        );
    });

    it('finds a conversation by title', () => {
        const host = render();

        filter(host, 'GATEWAY');

        expect(titles(host)).toEqual(['Gateway restarts']);
        expect(host.querySelectorAll('ui-side-panel-section')).toHaveLength(1);
    });

    it('says so when no conversation matches', () => {
        const host = render();

        filter(host, 'nothing like it');

        expect(titles(host)).toEqual([]);
        expect(host.querySelector('[data-testid="chat-list-empty"]')!.textContent).toContain(
            'AGENT_CHAT.FILTER_EMPTY'
        );
    });

    it('pins and deletes from the row menu after confirming', () => {
        const host = render();
        const actions = host.querySelectorAll<HTMLElement>(
            '[data-testid="chat-list-item-actions"]'
        );

        actions[1].click();
        fixture.detectChanges();
        expect(actions[1].getAttribute('aria-expanded')).toBe('true');
        menuItem('AGENT_CHAT.PIN').click();
        fixture.detectChanges();

        expect(fixture.componentInstance.pins).toEqual([{ idConversation: 2, isPinned: true }]);

        actions[0].click();
        fixture.detectChanges();
        expect(menuItem('AGENT_CHAT.UNPIN')).toBeDefined();
        menuItem('AGENT_CHAT.DELETE').click();
        fixture.detectChanges();

        const panel = document.querySelector('.ui-confirm-panel')!;
        expect(panel.textContent).toContain('AGENT_CHAT.DELETE_CONFIRM');
        expect(fixture.componentInstance.deletes).toEqual([]);

        // Template order: reject first, accept second.
        panel.querySelectorAll<HTMLElement>('button')[1].click();
        fixture.detectChanges();

        expect(fixture.componentInstance.deletes).toEqual([1]);
    });

    it('reaches the row menu with the keyboard', async () => {
        const host = render();
        const row = host.querySelectorAll<HTMLElement>('[data-testid="chat-list-item"]')[1];
        const actions = host.querySelectorAll<HTMLElement>(
            '[data-testid="chat-list-item-actions"]'
        )[1];

        row.focus();
        await userEvent.tab();
        expect(document.activeElement).toBe(actions);
        expect(getComputedStyle(actions.parentElement!).opacity).toBe('1');

        await userEvent.keyboard('{Enter}');
        fixture.detectChanges();
        await new Promise(resolve => setTimeout(resolve));

        expect(document.activeElement?.textContent).toContain('AGENT_CHAT.PIN');
    });
});

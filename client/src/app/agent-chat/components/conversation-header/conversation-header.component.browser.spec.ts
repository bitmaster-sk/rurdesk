import { TemplateRef } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { provideRouter, RouterModule } from '@angular/router';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { page } from '@vitest/browser/context';
import { AgentStatus, AgentConversationDetail } from '../../model/agent-conversation.model';
import {
    AvatarStub,
    TablerIconStub,
    UiButtonStub,
    UiConfirmStub,
    UiDialogStub,
    UiSelectStub,
    UiTooltipStub
} from 'src/testing/stubs';
import { Fixtures } from 'src/testing/fixtures';
import { AgentConversationSettings } from '../../model/agent-conversation.model';
import { ChatModelSettingsComponent } from '../chat-model-settings/chat-model-settings.component';
import { UiModule } from '../../../ui/ui.module';
import { ConversationHeaderComponent } from './conversation-header.component';
import { AgentStatusState } from '../../constants/agent-status-state.enum';

function detail(overrides: {
    status?: Partial<AgentStatus>;
    isLocked?: boolean;
    tokens?: number;
    cost?: { usd: number | null; estimated: boolean };
    hasMain?: boolean;
    pinnedAt?: string;
    model?: string | null;
}): AgentConversationDetail {
    return {
        conversation: {
            idConversation: 4,
            idUser: 1,
            idUserAgent: 9,
            idProject: overrides.hasMain === false ? null : 7,
            idsProjectContext: [8],
            idRun: 30,
            title: 'Why is the worktree kept?',
            headCommit: null,
            isLocked: overrides.isLocked ?? true,
            taskDraftRequest: null,
            readAt: null,
            pinnedAt: overrides.pinnedAt ?? null,
            model: overrides.model ?? null,
            lastAgentMessageAt: null,
            createdAt: '2026-11-16T10:00:00Z',
            updatedAt: '2026-11-16T10:00:00Z'
        },
        projects:
            overrides.hasMain === false
                ? [{ idProject: 8, name: 'gateway', isMain: false }]
                : [
                      { idProject: 7, name: 'rurdesk', isMain: true },
                      { idProject: 8, name: 'gateway', isMain: false }
                  ],
        agent: Fixtures.agent({ idUser: 9, name: 'Agent 003' }),
        agentStatus: {
            state: AgentStatusState.Idle,
            idIssuePublic: null,
            idProject: null,
            stage: null,
            queuePosition: null,
            idTask: null,
            ...overrides.status
        },
        agentModels: {
            models: ['anthropic/claude-opus', 'anthropic/claude-sonnet', 'ollama/llama3.3'],
            modelDefault: 'anthropic/claude-sonnet'
        },
        stats:
            overrides.tokens === undefined
                ? null
                : {
                      totalTokensUsed: overrides.tokens,
                      totalDurationMs: 0,
                      totalToolCallsCount: 0,
                      totalInputTokens: 0,
                      totalOutputTokens: 0,
                      totalCacheReadTokens: 0,
                      totalCacheWriteTokens: 0,
                      totalCostUsd: overrides.cost?.usd ?? null,
                      costEstimated: overrides.cost?.estimated ?? false,
                      attemptsPerStage: {},
                      failedAttempts: 0,
                      tokensByModel: {}
                  }
    };
}

describe('ConversationHeaderComponent (browser)', () => {
    let fixture: ComponentFixture<ConversationHeaderComponent>;

    beforeEach(async () => {
        await page.viewport(1280, 800);
        await TestBed.configureTestingModule({
            declarations: [ConversationHeaderComponent, ChatModelSettingsComponent],
            imports: [
                TranslateModule.forRoot(),
                FormsModule,
                ReactiveFormsModule,
                RouterModule,
                AvatarStub,
                TablerIconStub,
                UiButtonStub,
                UiConfirmStub,
                UiDialogStub,
                UiSelectStub,
                UiTooltipStub
            ],
            providers: [provideRouter([])]
        }).compileComponents();
        const translate = TestBed.inject(TranslateService);
        translate.setTranslation('en', {
            AGENT_CHAT: {
                STATUS: {
                    IDLE: 'Available',
                    ANSWERING: 'Answering you…',
                    BUSY: 'Working on another task',
                    BUSY_ISSUE: 'Working on',
                    QUEUED: 'Queued, position {{position}}',
                    QUEUED_ISSUE: 'Queued, position {{position}}, after'
                },
                TOKENS: '{{tokens}} tokens',
                AGENT: 'Agent',
                ADD_PROJECT: 'Add project',
                MAKE_MAIN: 'Make main',
                MAIN_HELP: {
                    TITLE: 'Main project',
                    MAIN: 'Code, tasks and wiki',
                    CONTEXT: 'Other projects',
                    LOCK: 'The main project cannot change after the first message.'
                }
            },
            AGENT: { STATS: { COST_ESTIMATED_HINT: 'Estimate, not what you pay.' } }
        });
        translate.use('en');
    });

    function render(value: AgentConversationDetail): HTMLElement {
        fixture = TestBed.createComponent(ConversationHeaderComponent);
        fixture.componentRef.setInput('detail', value);
        fixture.detectChanges();
        return fixture.nativeElement as HTMLElement;
    }

    function status(host: HTMLElement): string {
        return host.querySelector('[data-testid="chat-header-status"]')!.textContent.trim();
    }

    function issueLink(host: HTMLElement): HTMLAnchorElement | null {
        return host.querySelector('[data-testid="chat-header-status-issue"]');
    }

    function picker(testId: string): UiSelectStub {
        return fixture.debugElement
            .query(debug => debug.attributes['data-testid'] === testId)
            .injector.get(UiSelectStub);
    }

    it('offers to pin an unpinned conversation and to unpin a pinned one', () => {
        const pins: boolean[] = [];
        const host = render(detail({}));
        fixture.componentInstance.pinChange.subscribe(isPinned => pins.push(isPinned));
        const button = () => host.querySelector<HTMLElement>('[data-testid="chat-header-pin"]')!;
        const label = () =>
            fixture.debugElement
                .query(debug => debug.attributes['data-testid'] === 'chat-header-pin')
                .injector.get(UiButtonStub)
                .ariaLabel();

        expect(label()).toBe('AGENT_CHAT.PIN');
        button().click();
        fixture.componentRef.setInput('detail', detail({ pinnedAt: '2026-11-16T11:00:00Z' }));
        fixture.detectChanges();

        expect(label()).toBe('AGENT_CHAT.UNPIN');
        button().click();
        expect(pins).toEqual([true, false]);
    });

    it('asks before deleting and emits only once the owner confirms', () => {
        const deletes: unknown[] = [];
        const host = render(detail({}));
        fixture.componentInstance.deleteConversation.subscribe(() => deletes.push(true));
        const button = fixture.debugElement.query(
            debug => debug.attributes['data-testid'] === 'chat-header-delete'
        );

        host.querySelector<HTMLElement>('[data-testid="chat-header-delete"]')!.click();
        expect(deletes).toEqual([]);
        expect(button.injector.get(UiConfirmStub).confirmText()).toBe('AGENT_CHAT.DELETE_CONFIRM');
        expect(button.injector.get(UiButtonStub).severity()).toBe('secondary');

        button.injector.get(UiConfirmStub).confirmed.emit();
        expect(deletes).toEqual([true]);
    });

    it('says the agent is available when it is idle', () => {
        const host = render(detail({}));

        expect(status(host)).toBe('Available');
        expect(issueLink(host)).toBeNull();
    });

    it('says the agent is answering while this conversation runs', () => {
        const host = render(detail({ status: { state: AgentStatusState.Busy, stage: 'chat' } }));

        expect(status(host)).toBe('Answering you…');
    });

    it('names the issue the agent works on when the owner can read it', () => {
        const host = render(
            detail({
                status: {
                    state: AgentStatusState.Busy,
                    stage: 'implementation',
                    idProject: 7,
                    idIssuePublic: 42
                }
            })
        );

        expect(status(host)).toContain('Working on');
        expect(issueLink(host)!.textContent.trim()).toBe('#42');
        expect(issueLink(host)!.getAttribute('href')).toBe('/project/7/issue/42');
    });

    it('only says the agent is busy when the issue is not readable', () => {
        const host = render(detail({ status: { state: AgentStatusState.Busy } }));

        expect(status(host)).toBe('Working on another task');
        expect(issueLink(host)).toBeNull();
    });

    it('shows the queue position of a waiting turn', () => {
        const host = render(
            detail({ status: { state: AgentStatusState.Queued, queuePosition: 2 } })
        );

        expect(status(host)).toBe('Queued, position 2');
        expect(issueLink(host)).toBeNull();
    });

    it('shows the queue position and the issue ahead when it is readable', () => {
        const host = render(
            detail({
                status: {
                    state: AgentStatusState.Queued,
                    queuePosition: 1,
                    idProject: 7,
                    idIssuePublic: 42
                }
            })
        );

        expect(status(host)).toContain('Queued, position 1, after');
        expect(issueLink(host)!.textContent.trim()).toBe('#42');
    });

    it('shows the tokens used so far', () => {
        const host = render(detail({ tokens: 24_180 }));

        expect(host.querySelector('[data-testid="chat-header-tokens"]')!.textContent.trim()).toBe(
            '24.2k tokens'
        );
    });

    it('shows the cost next to the tokens when it is known', () => {
        const host = render(detail({ tokens: 24_180, cost: { usd: 0.41, estimated: false } }));

        const cost = host.querySelector('[data-testid="chat-header-cost"]')!;
        expect(cost.textContent.trim()).toBe('$0.41');
    });

    it('marks an estimated cost and explains it in a tooltip', () => {
        const host = render(detail({ tokens: 24_180, cost: { usd: 0.41, estimated: true } }));

        const cost = host.querySelector<HTMLElement>('[data-testid="chat-header-cost"]')!;
        expect(cost.textContent.trim()).toBe('≈ $0.41');
        const tooltip = fixture.debugElement
            .query(debug => debug.attributes['data-testid'] === 'chat-header-cost')
            .injector.get(UiTooltipStub);
        expect(tooltip.uiTooltip()).toBe('Estimate, not what you pay.');
    });

    it('shows no cost while it is unknown', () => {
        const host = render(detail({ tokens: 24_180 }));

        expect(host.querySelector('[data-testid="chat-header-cost"]')).toBeNull();
        expect(host.textContent).not.toContain('$');
    });

    it('keeps the main project fixed once the conversation is locked', () => {
        const host = render(detail({ isLocked: true }));
        const [main, context] = Array.from(
            host.querySelectorAll<HTMLElement>('[data-testid="chat-chip"]')
        );

        expect(main.querySelector('[data-testid="chat-chip-remove"]')).toBeNull();
        const remove = context.querySelector<HTMLButtonElement>(
            '[data-testid="chat-chip-remove"]'
        )!;
        expect(remove.disabled).toBe(false);
    });

    it('explains the main project in a tooltip reachable by keyboard', () => {
        const host = render(detail({ isLocked: true }));
        const label = host.querySelector<HTMLElement>('[data-testid="chat-chip-main"]')!;

        expect(label.tabIndex).toBe(0);
        const help = fixture.debugElement
            .query(debug => debug.attributes['data-testid'] === 'chat-chip-main')
            .injector.get(UiTooltipStub)
            .uiTooltip();
        expect(help).toBeInstanceOf(TemplateRef);
        const view = (help as TemplateRef<unknown>).createEmbeddedView({});
        view.detectChanges();
        const text = view.rootNodes.map((node: Node) => node.textContent).join(' ');
        view.destroy();
        expect(text).toContain('Main project');
        expect(text).toContain('Code, tasks and wiki');
        expect(text).toContain('Other projects');
        expect(text).toContain('The main project cannot change after the first message.');
    });

    it('lets the owner change the main project before the first message', () => {
        const host = render(detail({ isLocked: false }));
        const main = host.querySelector<HTMLButtonElement>('[data-testid="chat-chip-remove"]')!;

        expect(main.disabled).toBe(false);
    });

    function makeMainButtons(host: HTMLElement): HTMLButtonElement[] {
        return Array.from(
            host.querySelectorAll<HTMLButtonElement>('[data-testid="chat-chip-make-main"]')
        );
    }

    it('offers Make main on context chips only while there is no main project', () => {
        expect(makeMainButtons(render(detail({ isLocked: false })))).toEqual([]);
        fixture.destroy();

        const buttons = makeMainButtons(render(detail({ isLocked: false, hasMain: false })));

        expect(buttons.map(button => button.textContent.trim())).toEqual(['Make main']);
    });

    it('asks to attach the context project that is made main', () => {
        const host = render(detail({ isLocked: false, hasMain: false }));
        const attached: number[] = [];
        fixture.componentInstance.attachProject.subscribe(id => attached.push(id));

        makeMainButtons(host)[0].click();

        expect(attached).toEqual([8]);
    });

    // The agent is picked when the conversation starts and never changes.
    it('names the agent without offering to change it', () => {
        const host = render(detail({ isLocked: false }));
        const agent = host.querySelector('[data-testid="chat-header-agent"]')!;

        expect(agent.textContent).toContain('Agent 003');
        expect(agent.getAttribute('aria-label')).toBe('Agent');
        expect(host.querySelector('[data-testid="chat-agent-picker"]')).toBeNull();
    });

    it('labels the add-project picker and announces status changes', () => {
        const host = render(detail({}));
        const label = host.querySelector<HTMLLabelElement>('label[for="chat-add-project"]')!;

        expect(label.textContent.trim()).toBe('Add project');
        expect(picker('chat-add-project').inputId()).toBe('chat-add-project');
        const statusLine = host.querySelector('[data-testid="chat-header-status"]')!;
        expect(statusLine.getAttribute('role')).toBe('status');
        expect(statusLine.getAttribute('aria-live')).toBe('polite');
    });

    it('asks to remove a context project', () => {
        const host = render(detail({}));
        const removed: number[] = [];
        fixture.componentInstance.detachProject.subscribe(id => removed.push(id));

        host.querySelectorAll('[data-testid="chat-chip"]')[1]
            .querySelector<HTMLButtonElement>('[data-testid="chat-chip-remove"]')!
            .click();

        expect(removed).toEqual([8]);
    });

    it('offers only projects not yet attached and asks to attach the picked one', () => {
        render(detail({}));
        fixture.componentRef.setInput('projects', [
            Fixtures.project({ idProject: 7, name: 'rurdesk' }),
            Fixtures.project({ idProject: 8, name: 'gateway' }),
            Fixtures.project({ idProject: 9, name: 'site' })
        ]);
        fixture.detectChanges();
        const attached: number[] = [];
        fixture.componentInstance.attachProject.subscribe(id => attached.push(id));
        const add = picker('chat-add-project');

        expect(add.options()!.map(p => (p as { idProject: number }).idProject)).toEqual([9]);
        add.pick(9);

        expect(attached).toEqual([9]);
    });

    it('shows why a project could not be attached', () => {
        render(detail({}));
        fixture.componentRef.setInput('projectError', 'error.agent_not_in_project');
        fixture.detectChanges();

        expect(
            (fixture.nativeElement as HTMLElement).querySelector(
                '[data-testid="chat-project-error"]'
            )!.textContent
        ).toContain('error.agent_not_in_project');
    });
    describe('model', () => {
        // The form control writes the value and the disabled state into the select asynchronously.
        async function renderSettled(value: AgentConversationDetail): Promise<void> {
            render(value);
            await fixture.whenStable();
            fixture.detectChanges();
        }

        function changes(): AgentConversationSettings[] {
            const changed: AgentConversationSettings[] = [];
            fixture.componentInstance.settingsChange.subscribe(settings => changed.push(settings));
            return changed;
        }

        function lockHint(): string {
            return fixture.debugElement
                .query(debug => debug.attributes['data-testid'] === 'chat-model-settings')
                .injector.get(UiTooltipStub)
                .uiTooltip() as string;
        }

        it('shows the current model and asks to change it', async () => {
            await renderSettled(detail({ model: 'anthropic/claude-opus' }));
            const changed = changes();

            expect(picker('chat-model-select').value).toBe('anthropic/claude-opus');
            picker('chat-model-select').pick('ollama/llama3.3');
            picker('chat-model-select').pick(null);

            expect(changed).toEqual([{ model: 'ollama/llama3.3' }, { model: null }]);
        });

        it('offers the default model with its name and no effort', async () => {
            await renderSettled(detail({}));

            expect(picker('chat-model-select').options()).toEqual([
                { value: null, label: 'AGENT_CHAT.MODEL_DEFAULT' },
                { value: 'anthropic/claude-opus', label: 'anthropic/claude-opus' },
                { value: 'anthropic/claude-sonnet', label: 'anthropic/claude-sonnet' },
                { value: 'ollama/llama3.3', label: 'ollama/llama3.3' }
            ]);
            const host = fixture.nativeElement as HTMLElement;
            expect(host.querySelector('app-agent-effort-picker')).toBeNull();
            expect(host.querySelector('[data-testid^="chat-effort"]')).toBeNull();
        });

        it('disables the model with a hint while the agent is answering', async () => {
            await renderSettled(
                detail({ status: { state: AgentStatusState.Busy, stage: 'chat' } })
            );

            expect(picker('chat-model-select').isDisabled).toBe(true);
            expect(lockHint()).toBe('AGENT_CHAT.SETTINGS_LOCKED');
        });

        it('lets the owner change it while the turn waits in the queue', async () => {
            await renderSettled(
                detail({ status: { state: AgentStatusState.Queued, queuePosition: 2 } })
            );

            expect(picker('chat-model-select').isDisabled).toBe(false);
            expect(lockHint()).toBe('');
        });
    });
    describe('at phone width', () => {
        beforeEach(async () => {
            await page.viewport(390, 844);
        });

        afterEach(async () => {
            await page.viewport(1280, 800);
        });

        function box(host: HTMLElement, testId: string): DOMRect {
            return host.querySelector(`[data-testid="${testId}"]`)!.getBoundingClientRect();
        }

        it('offers a back control before the title that tells the page to return to the list', () => {
            const backs: unknown[] = [];
            const host = render(detail({}));
            fixture.componentInstance.back.subscribe(() => backs.push(true));

            const back = host.querySelector<HTMLElement>('[data-testid="chat-header-back"]')!;
            expect(back.getBoundingClientRect().right).toBeLessThanOrEqual(
                box(host, 'chat-header-title').left
            );
            back.click();

            expect(backs).toEqual([true]);
        });

        // The component specs load no global stylesheet: give the spacing tokens the header uses.
        function withSpacingTokens(): void {
            const style = document.createElement('style');
            style.textContent =
                ':root { --ui-space: 0.5rem; --ui-space-s: 0.25rem; --ui-space-l: 1rem; }';
            document.head.appendChild(style);
            onTestFinished(() => style.remove());
        }

        function crowded(): AgentConversationDetail {
            const long = detail({ tokens: 92200, cost: { usd: 0.41, estimated: false } });
            long.conversation.title = 'A very long conversation title '.repeat(10);
            long.agent = Fixtures.agent({ idUser: 9, name: 'Agent with a very long name 003' });
            long.projects = Array.from({ length: 6 }, (_, index) => ({
                idProject: index + 1,
                name: `Project number ${index + 1}`,
                isMain: index === 0
            }));
            return long;
        }

        async function renderSettled(value: AgentConversationDetail): Promise<HTMLElement> {
            const host = render(value);
            await fixture.whenStable();
            fixture.detectChanges();
            return host;
        }

        async function openSettings(host: HTMLElement): Promise<void> {
            host.querySelector<HTMLElement>('[data-testid="chat-header-settings"]')!.click();
            await fixture.whenStable();
            fixture.detectChanges();
        }

        it('keeps the title with the settings, pin and delete on one row above one meta line', () => {
            withSpacingTokens();
            const host = render(detail({ tokens: 1200, cost: { usd: 0.41, estimated: false } }));

            const title = box(host, 'chat-header-title');
            ['chat-header-settings', 'chat-header-pin', 'chat-header-delete'].forEach(testId => {
                expect(box(host, testId).top).toBeLessThan(title.bottom);
            });
            const meta = [
                'chat-header-agent',
                'chat-header-status',
                'chat-header-tokens',
                'chat-header-cost'
            ].map(testId => box(host, testId));
            meta.forEach(rect => {
                expect(rect.top).toBeGreaterThanOrEqual(title.bottom);
                expect(
                    Math.abs(rect.top + rect.height / 2 - (meta[0].top + meta[0].height / 2))
                ).toBeLessThan(8);
            });
        });

        it('stays two rows under 120px and inside the width however long the title, agent and chips are', async () => {
            withSpacingTokens();
            const host = await renderSettled(crowded());

            expect(host.getBoundingClientRect().height).toBeLessThanOrEqual(120);
            expect(host.scrollWidth).toBeLessThanOrEqual(host.clientWidth);
            expect(host.getBoundingClientRect().right).toBeLessThanOrEqual(390);
            const meta = host.querySelector<HTMLElement>('.chat-header-meta')!;
            expect(meta.scrollWidth).toBeLessThanOrEqual(meta.clientWidth);
            expect(meta.getBoundingClientRect().height).toBeLessThan(32);
        });

        it('keeps the model and projects out of the header until the settings open', async () => {
            const host = await renderSettled(detail({}));

            ['chat-model-settings', 'chat-chip', 'chat-add-project'].forEach(testId => {
                expect(host.querySelector(`[data-testid="${testId}"]`)).toBeNull();
            });
        });

        it('opens the model and projects in a panel from the settings button', async () => {
            const host = await renderSettled(detail({}));

            await openSettings(host);

            ['chat-model-select', 'chat-add-project'].forEach(testId => {
                expect(host.querySelector(`[data-testid="${testId}"]`)).not.toBeNull();
            });
            expect(host.querySelectorAll('[data-testid="chat-chip"]').length).toBe(2);
            expect(
                fixture.debugElement
                    .query(debug => debug.attributes['data-testid'] === 'chat-header-settings')
                    .injector.get(UiButtonStub)
                    .ariaLabel()
            ).toBe('AGENT_CHAT.CONVERSATION_SETTINGS');
        });

        it('changes the model from the panel', async () => {
            const host = await renderSettled(detail({}));
            const changed: AgentConversationSettings[] = [];
            fixture.componentInstance.settingsChange.subscribe(value => changed.push(value));
            await openSettings(host);

            picker('chat-model-select').pick('anthropic/claude-opus');

            expect(changed).toEqual([{ model: 'anthropic/claude-opus' }]);
            expect(host.querySelector('[data-testid^="chat-effort"]')).toBeNull();
        });

        it('locks the model in the panel while the agent answers', async () => {
            const host = await renderSettled(
                detail({ status: { state: AgentStatusState.Busy, stage: 'chat' } })
            );
            await openSettings(host);

            expect(picker('chat-model-select').isDisabled).toBe(true);
        });

        it('asks to attach and detach projects from the panel', async () => {
            const host = await renderSettled(detail({ hasMain: false }));
            const attached: number[] = [];
            fixture.componentInstance.attachProject.subscribe(id => attached.push(id));
            await openSettings(host);

            host.querySelector<HTMLElement>('[data-testid="chat-chip-make-main"]')!.click();

            expect(attached).toEqual([8]);
        });
    });

    describe('at desktop width', () => {
        function box(host: HTMLElement, selector: string): DOMRect {
            return host.querySelector(selector)!.getBoundingClientRect();
        }

        it('puts the title with the pin and delete on top and the agent, usage and run settings on one line below', async () => {
            await page.viewport(1440, 900);
            const host = render(detail({ tokens: 1200, cost: { usd: 0.41, estimated: false } }));
            await fixture.whenStable();
            fixture.detectChanges();

            const title = box(host, '[data-testid="chat-header-title"]');
            const pin = box(host, '[data-testid="chat-header-pin"]');
            const remove = box(host, '[data-testid="chat-header-delete"]');
            expect(pin.left).toBeGreaterThanOrEqual(title.right);
            expect(remove.right).toBeGreaterThanOrEqual(pin.right);
            expect(remove.right).toBeLessThanOrEqual(host.getBoundingClientRect().right);
            expect(Math.abs(pin.top - title.top)).toBeLessThan(title.height);

            const meta = [
                'chat-header-agent',
                'chat-header-status',
                'chat-header-tokens',
                'chat-header-cost'
            ]
                .map(testId => box(host, `[data-testid="${testId}"]`))
                .concat(box(host, '[data-testid="chat-model-settings"]'));
            meta.forEach(rect => {
                expect(rect.top).toBeGreaterThanOrEqual(title.bottom);
                expect(
                    Math.abs(rect.top + rect.height / 2 - (meta[0].top + meta[0].height / 2))
                ).toBeLessThan(12);
            });
            const chips = box(host, '[data-testid="chat-chip"]');
            expect(chips.top).toBeGreaterThanOrEqual(Math.max(...meta.map(rect => rect.bottom)));
        });

        it('keeps the model and projects in the header without a settings button', async () => {
            await page.viewport(1280, 800);
            const host = render(detail({}));
            await fixture.whenStable();
            fixture.detectChanges();

            expect(host.querySelector('[data-testid="chat-header-settings"]')).toBeNull();
            expect(host.querySelector('[data-testid="chat-model-settings"]')).not.toBeNull();
            expect(host.querySelector('[data-testid="chat-chip"]')).not.toBeNull();
        });

        it('has no back control', async () => {
            await page.viewport(1280, 800);
            const host = render(detail({}));

            expect(
                host.querySelector('[data-testid="chat-header-back"]')!.getBoundingClientRect()
                    .width
            ).toBe(0);
        });
    });
});

describe('ConversationHeaderComponent model (browser, real select)', () => {
    let fixture: ComponentFixture<ConversationHeaderComponent>;

    beforeEach(async () => {
        await page.viewport(1280, 800);
        await TestBed.configureTestingModule({
            declarations: [ConversationHeaderComponent, ChatModelSettingsComponent],
            imports: [
                UiModule,
                FormsModule,
                ReactiveFormsModule,
                RouterModule,
                TranslateModule.forRoot(),
                AvatarStub,
                TablerIconStub
            ],
            providers: [provideRouter([]), provideNoopAnimations()]
        }).compileComponents();
        const translate = TestBed.inject(TranslateService);
        translate.setTranslation('en', {
            AGENT_CHAT: { MODEL_DEFAULT: 'Default ({{model}})' }
        });
        translate.use('en');
    });

    async function settle(): Promise<void> {
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();
    }

    function trigger(testId: string): HTMLElement {
        return (fixture.nativeElement as HTMLElement).querySelector(
            `[data-testid="${testId}"] .ui-select-trigger`
        )!;
    }

    async function pick(testId: string, label: string): Promise<void> {
        trigger(testId).click();
        fixture.detectChanges();
        Array.from(document.querySelectorAll<HTMLElement>('.ui-select-panel__option'))
            .find(option => option.textContent?.trim() === label)!
            .click();
        await settle();
    }

    it('shows the stored model again when the change is refused', async () => {
        fixture = TestBed.createComponent(ConversationHeaderComponent);
        fixture.componentRef.setInput('detail', detail({}));
        await settle();
        const changed: AgentConversationSettings[] = [];
        fixture.componentInstance.settingsChange.subscribe(settings => changed.push(settings));

        await pick('chat-model-select', 'anthropic/claude-opus');
        expect(changed).toEqual([{ model: 'anthropic/claude-opus' }]);

        // The page reloads the detail after a refusal: the stored model did not change.
        fixture.componentRef.setInput('detail', detail({}));
        await settle();

        expect(trigger('chat-model-select').textContent).toContain(
            'Default (anthropic/claude-sonnet)'
        );
    });

    describe('settings panel at phone width', () => {
        beforeEach(async () => {
            await page.viewport(390, 844);
        });

        afterEach(async () => {
            await page.viewport(1280, 800);
        });

        async function open(): Promise<void> {
            fixture = TestBed.createComponent(ConversationHeaderComponent);
            fixture.componentRef.setInput('detail', detail({}));
            await settle();
            (fixture.nativeElement as HTMLElement)
                .querySelector<HTMLElement>('[data-testid="chat-header-settings"]')!
                .click();
            await settle();
        }

        const panel = () => document.querySelector<HTMLElement>('.ui-dialog');

        it('opens as a panel inside the screen with the model and projects', async () => {
            await open();

            const box = panel()!.getBoundingClientRect();
            expect(box.left).toBeGreaterThanOrEqual(0);
            expect(box.right).toBeLessThanOrEqual(390);
            ['chat-model-select', 'chat-add-project'].forEach(testId => {
                expect(panel()!.querySelector(`[data-testid="${testId}"]`)).not.toBeNull();
            });
            expect(panel()!.querySelectorAll('[data-testid="chat-chip"]').length).toBe(2);
            expect(panel()!.scrollWidth).toBeLessThanOrEqual(panel()!.clientWidth);
        });

        it('closes on Escape, on the close button and on a tap outside', async () => {
            await open();
            document.body.dispatchEvent(
                new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })
            );
            await settle();
            expect(panel()).toBeNull();

            await open();
            document.querySelector<HTMLElement>('.ui-dialog__close')!.click();
            await settle();
            expect(panel()).toBeNull();

            await open();
            document.querySelector<HTMLElement>('.ui-dialog__backdrop')!.click();
            await settle();
            expect(panel()).toBeNull();
        });

        it('shows the stored model again when a change from the panel is refused', async () => {
            await open();
            const changed: AgentConversationSettings[] = [];
            fixture.componentInstance.settingsChange.subscribe(settings => changed.push(settings));
            const trigger = () =>
                panel()!.querySelector<HTMLElement>(
                    '[data-testid="chat-model-select"] .ui-select-trigger'
                )!;

            trigger().click();
            fixture.detectChanges();
            Array.from(document.querySelectorAll<HTMLElement>('.ui-select-panel__option'))
                .find(option => option.textContent?.trim() === 'anthropic/claude-opus')!
                .click();
            await settle();
            expect(changed).toEqual([{ model: 'anthropic/claude-opus' }]);

            fixture.componentRef.setInput('detail', detail({}));
            await settle();

            expect(trigger().textContent).toContain('Default (anthropic/claude-sonnet)');
            expect(panel()!.querySelector('[data-testid^="chat-effort"]')).toBeNull();
        });

        // The component specs load no global stylesheet: give the spacing tokens the header uses.
        function withSpacingTokens(): void {
            const style = document.createElement('style');
            style.textContent =
                ':root { --ui-space: 0.5rem; --ui-space-s: 0.25rem; --ui-space-l: 1rem; }';
            document.head.appendChild(style);
            onTestFinished(() => style.remove());
        }

        it('lines up the arrow of the back control with the agent avatar below it', async () => {
            withSpacingTokens();
            fixture = TestBed.createComponent(ConversationHeaderComponent);
            fixture.componentRef.setInput('detail', detail({}));
            await settle();
            const host = fixture.nativeElement as HTMLElement;

            const back = host
                .querySelector('[data-testid="chat-header-back"] .ui-button')!
                .getBoundingClientRect();
            const avatar = host
                .querySelector('[data-testid="chat-header-agent"] app-avatar')!
                .getBoundingClientRect();
            // The 18px arrow icon sits centred in the button; its stroke starts 3px into the icon.
            const arrowLeft = back.left + (back.width - 18) / 2 + 3;

            expect(back.width).toBeGreaterThanOrEqual(44);
            expect(Math.abs(arrowLeft - avatar.left)).toBeLessThanOrEqual(1);
            expect(back.left).toBeGreaterThanOrEqual(0);
        });

        it('labels the context above the projects like the model', async () => {
            await open();

            const model = panel()!.querySelector<HTMLElement>('label[for="chat-model-select"]')!;
            const context = panel()!.querySelector<HTMLElement>(
                '[data-testid="chat-context-label"]'
            )!;
            const chip = panel()!.querySelector<HTMLElement>('[data-testid="chat-chip"]')!;
            const modelStyle = getComputedStyle(model);
            const contextStyle = getComputedStyle(context);

            expect(context.getBoundingClientRect().bottom).toBeLessThanOrEqual(
                chip.getBoundingClientRect().top
            );
            expect(context.getBoundingClientRect().left).toBe(model.getBoundingClientRect().left);
            ['fontSize', 'fontWeight', 'textTransform', 'letterSpacing', 'color'].forEach(prop => {
                expect(
                    contextStyle.getPropertyValue(
                        prop.replace(/[A-Z]/g, c => '-' + c.toLowerCase())
                    )
                ).toBe(
                    modelStyle.getPropertyValue(prop.replace(/[A-Z]/g, c => '-' + c.toLowerCase()))
                );
            });
        });
    });
});

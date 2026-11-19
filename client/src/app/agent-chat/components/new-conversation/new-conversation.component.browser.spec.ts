import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { page } from '@vitest/browser/context';
import { Fixtures } from 'src/testing/fixtures';
import { TablerIconStub } from 'src/testing/stubs';
import { AgentRunApi } from '../../../agent/api/agent-run.api.service';
import { AvatarComponent } from '../../../shared/avatar/avatar.component';
import { UserDropdownComponent } from '../../../shared/user-dropdown/user-dropdown.component';
import { UiModule } from '../../../ui/ui.module';
import { AgentConversationApi } from '../../api/agent-conversation.api.service';
import { AgentConversationAgent } from '../../model/agent-conversation.model';
import { ChatModelSettingsComponent } from '../chat-model-settings/chat-model-settings.component';
import { NewConversationComponent } from './new-conversation.component';

const ADA: AgentConversationAgent = {
    ...Fixtures.agent({ idUser: 11, name: 'Ada Agent' }),
    models: ['anthropic/claude-opus', 'anthropic/claude-sonnet', 'ollama/llama3.3'],
    modelDefault: 'anthropic/claude-sonnet'
};
const BOB: AgentConversationAgent = {
    ...Fixtures.agent({ idUser: 12, name: 'Bob Agent' }),
    models: ['openai/gpt-5'],
    modelDefault: 'openai/gpt-5'
};
const ALPHA = Fixtures.project({ idProject: 1, name: 'Alpha' });
const BETA = Fixtures.project({ idProject: 2, name: 'Beta' });
const GAMMA = Fixtures.project({ idProject: 3, name: 'Gamma' });

describe('NewConversationComponent (browser)', () => {
    let api: {
        loadAgents$: ReturnType<typeof vi.fn>;
        insert$: ReturnType<typeof vi.fn>;
        insertProject$: ReturnType<typeof vi.fn>;
    };

    beforeEach(async () => {
        api = {
            loadAgents$: vi.fn().mockReturnValue(of([ADA, BOB])),
            insert$: vi.fn().mockReturnValue(of({ idConversation: 5 })),
            insertProject$: vi.fn().mockReturnValue(of(undefined))
        };
        await TestBed.configureTestingModule({
            declarations: [
                NewConversationComponent,
                ChatModelSettingsComponent,
                UserDropdownComponent,
                AvatarComponent
            ],
            imports: [
                UiModule,
                FormsModule,
                ReactiveFormsModule,
                TablerIconStub,
                TranslateModule.forRoot()
            ],
            providers: [
                provideNoopAnimations(),
                { provide: AgentConversationApi, useValue: api },
                { provide: AgentRunApi, useValue: {} }
            ]
        }).compileComponents();

        const translate = TestBed.inject(TranslateService);
        translate.setTranslation('en', {
            UI: { CLEAR: 'Clear', SEARCH: 'Search' },
            AGENT_CHAT: {
                MAIN_PROJECT: 'main',
                NEW: 'New',
                NEW_FORM: { NO_AGENTS: 'No agent is available here.', MAKE_MAIN: 'Make main' },
                MODEL: 'Model',
                MODEL_DEFAULT: 'Default ({{model}})'
            }
        });
        translate.use('en');
    });

    async function setup(
        inputs: { idProject?: number | null } = {}
    ): Promise<ComponentFixture<NewConversationComponent>> {
        const fixture = TestBed.createComponent(NewConversationComponent);
        fixture.componentRef.setInput('projects', [ALPHA, BETA, GAMMA]);
        if (inputs.idProject !== undefined) {
            fixture.componentRef.setInput('idProject', inputs.idProject);
        }
        // The first pass runs the project effect that loads the agents; ngModel
        // inside the picker writes its value asynchronously.
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();
        return fixture;
    }

    function openAgentPicker(fixture: ComponentFixture<NewConversationComponent>): void {
        const trigger = fixture.nativeElement.querySelector(
            'app-user-dropdown .ui-select-trigger'
        ) as HTMLElement;
        trigger.click();
        fixture.detectChanges();
    }

    function panelOptions(): HTMLElement[] {
        return Array.from(document.querySelectorAll('.ui-select-panel__option'));
    }

    it('renders the agent options with an avatar and the name', async () => {
        const fixture = await setup();
        openAgentPicker(fixture);

        const options = panelOptions();
        expect(options.map(option => option.textContent)).toEqual([
            expect.stringContaining('Ada Agent'),
            expect.stringContaining('Bob Agent')
        ]);
        options.forEach(option => expect(option.querySelector('app-avatar')).not.toBeNull());
    });

    it('selects the first agent and shows its avatar and name in the trigger', async () => {
        const fixture = await setup();

        const trigger = fixture.nativeElement.querySelector(
            'app-user-dropdown .ui-select-trigger'
        ) as HTMLElement;
        expect(trigger.querySelector('app-avatar')).not.toBeNull();
        expect(trigger.textContent).toContain('Ada Agent');
    });

    it('creates the conversation with the picked agent id', async () => {
        const fixture = await setup();
        openAgentPicker(fixture);
        const bob = panelOptions().find(option => option.textContent?.includes('Bob Agent'))!;
        bob.click();
        fixture.detectChanges();

        const created = vi.fn();
        fixture.componentInstance.created.subscribe(created);
        (fixture.nativeElement.querySelector('form') as HTMLFormElement).dispatchEvent(
            new Event('submit')
        );

        expect(api.insert$).toHaveBeenCalledWith({
            idUserAgent: 12,
            idProject: null,
            model: null
        });
        expect(created).toHaveBeenCalledWith({ idConversation: 5 });
    });

    it('shows the empty message and blocks starting when no agent is available', async () => {
        api.loadAgents$.mockReturnValue(of([]));
        const fixture = await setup();
        openAgentPicker(fixture);

        expect(panelOptions()).toHaveLength(0);
        expect(document.body.textContent).toContain('No agent is available here.');
        const start = fixture.nativeElement.querySelector(
            '[data-testid="new-conversation-start"]'
        ) as HTMLElement;
        expect(
            start.hasAttribute('disabled') || start.querySelector('button:disabled')
        ).toBeTruthy();
    });

    function pickProjects(fixture: ComponentFixture<NewConversationComponent>, ...names: string[]) {
        const trigger = fixture.nativeElement.querySelector(
            'ui-multiselect .ui-select-trigger'
        ) as HTMLElement;
        trigger.click();
        fixture.detectChanges();
        for (const name of names) {
            panelOptions()
                .find(option => option.textContent?.includes(name))!
                .click();
            fixture.detectChanges();
        }
    }

    function chips(fixture: ComponentFixture<NewConversationComponent>): HTMLElement[] {
        return Array.from(
            fixture.nativeElement.querySelectorAll('[data-testid="new-conversation-chip"]')
        );
    }

    function submit(fixture: ComponentFixture<NewConversationComponent>): void {
        (fixture.nativeElement.querySelector('form') as HTMLFormElement).dispatchEvent(
            new Event('submit')
        );
    }

    describe('projects', () => {
        it('marks the first chosen project as main', async () => {
            const fixture = await setup();
            pickProjects(fixture, 'Beta', 'Alpha');

            const [first, second] = chips(fixture);
            expect(first.textContent).toContain('Beta');
            expect(first.querySelector('[data-testid="new-conversation-main"]')).not.toBeNull();
            expect(second.textContent).toContain('Alpha');
            expect(second.querySelector('[data-testid="new-conversation-main"]')).toBeNull();
            expect(
                second.querySelector('[data-testid="new-conversation-make-main"]')
            ).not.toBeNull();
            expect(first.querySelector('[data-testid="new-conversation-make-main"]')).toBeNull();
        });

        it('lists the agents of the main project and reloads them when another becomes main', async () => {
            const fixture = await setup();
            pickProjects(fixture, 'Beta', 'Alpha');
            expect(api.loadAgents$).toHaveBeenLastCalledWith(2);

            (
                chips(fixture)[1].querySelector(
                    '[data-testid="new-conversation-make-main"]'
                ) as HTMLElement
            ).click();
            fixture.detectChanges();

            expect(api.loadAgents$).toHaveBeenLastCalledWith(1);
            expect(chips(fixture)[0].textContent).toContain('Alpha');
            expect(
                chips(fixture)[0].querySelector('[data-testid="new-conversation-main"]')
            ).not.toBeNull();
        });

        it('lists agents without a project when none is chosen', async () => {
            await setup();

            expect(api.loadAgents$).toHaveBeenLastCalledWith(null);
            expect(chips(await setup())).toHaveLength(0);
        });

        it('creates with the main project, then attaches the others in order', async () => {
            const calls: string[] = [];
            api.insert$.mockImplementation(() => {
                calls.push('create');
                return of({ idConversation: 5 });
            });
            api.insertProject$.mockImplementation((_: number, idProject: number) => {
                calls.push(`attach ${idProject}`);
                return of(undefined);
            });
            const fixture = await setup();
            pickProjects(fixture, 'Beta', 'Gamma', 'Alpha');
            const created = vi.fn();
            fixture.componentInstance.created.subscribe(created);

            submit(fixture);

            expect(api.insert$).toHaveBeenCalledWith({
                idUserAgent: 11,
                idProject: 2,
                model: null
            });
            expect(calls).toEqual(['create', 'attach 3', 'attach 1']);
            expect(api.insertProject$).toHaveBeenCalledWith(5, 3);
            expect(created).toHaveBeenCalledWith({ idConversation: 5 });
        });

        it('opens the conversation and names the projects the server refused', async () => {
            api.insertProject$.mockImplementation((_: number, idProject: number) =>
                idProject === 1 || idProject === 3
                    ? throwError(() => ({
                          status: 422,
                          error: { translateKey: 'error.agent_not_in_project' }
                      }))
                    : of(undefined)
            );
            const fixture = await setup();
            pickProjects(fixture, 'Beta', 'Gamma', 'Alpha');
            const created = vi.fn();
            const refused = vi.fn();
            fixture.componentInstance.created.subscribe(created);
            fixture.componentInstance.attachRefused.subscribe(refused);

            submit(fixture);

            expect(created).toHaveBeenCalledWith({ idConversation: 5 });
            expect(refused).toHaveBeenCalledWith([GAMMA, ALPHA]);
        });

        it('reports other attach failures apart from refusals', async () => {
            api.insertProject$.mockImplementation((_: number, idProject: number) =>
                idProject === 3
                    ? throwError(() => ({ status: 500 }))
                    : throwError(() => ({
                          status: 422,
                          error: { translateKey: 'error.agent_not_in_project' }
                      }))
            );
            const fixture = await setup();
            pickProjects(fixture, 'Beta', 'Gamma', 'Alpha');
            const created = vi.fn();
            const refused = vi.fn();
            const failed = vi.fn();
            fixture.componentInstance.created.subscribe(created);
            fixture.componentInstance.attachRefused.subscribe(refused);
            fixture.componentInstance.attachFailed.subscribe(failed);

            submit(fixture);

            expect(created).toHaveBeenCalledWith({ idConversation: 5 });
            expect(refused).toHaveBeenCalledWith([ALPHA]);
            expect(failed).toHaveBeenCalledWith([GAMMA]);
        });

        it('preselects the project the owner came from as the main one', async () => {
            const fixture = await setup({ idProject: 3 });

            expect(chips(fixture)).toHaveLength(1);
            expect(chips(fixture)[0].textContent).toContain('Gamma');
            expect(api.loadAgents$).toHaveBeenLastCalledWith(3);
        });
    });
    describe('model', () => {
        function trigger(
            fixture: ComponentFixture<NewConversationComponent>,
            testId: string
        ): HTMLElement | null {
            return fixture.nativeElement.querySelector(
                `[data-testid="${testId}"] .ui-select-trigger`
            ) as HTMLElement | null;
        }

        function pick(
            fixture: ComponentFixture<NewConversationComponent>,
            testId: string,
            label: string
        ): void {
            trigger(fixture, testId)!.click();
            fixture.detectChanges();
            panelOptions()
                .find(option => option.textContent?.trim() === label)!
                .click();
            fixture.detectChanges();
        }

        it("presets the model to the agent's default and offers no effort", async () => {
            const fixture = await setup();

            expect(trigger(fixture, 'chat-model-select')!.textContent).toContain(
                'Default (anthropic/claude-sonnet)'
            );
            expect(fixture.nativeElement.querySelector('[data-testid^="chat-effort"]')).toBeNull();
            expect(fixture.nativeElement.querySelector('.effort-picker__level')).toBeNull();
        });

        it('creates the conversation with the picked model', async () => {
            const fixture = await setup();
            pick(fixture, 'chat-model-select', 'anthropic/claude-opus');

            submit(fixture);

            expect(api.insert$).toHaveBeenCalledWith({
                idUserAgent: 11,
                idProject: null,
                model: 'anthropic/claude-opus'
            });
        });

        it("resets the model to the new agent's default", async () => {
            const fixture = await setup();
            pick(fixture, 'chat-model-select', 'anthropic/claude-opus');

            openAgentPicker(fixture);
            panelOptions()
                .find(option => option.textContent?.includes('Bob Agent'))!
                .click();
            fixture.detectChanges();
            await fixture.whenStable();
            fixture.detectChanges();

            expect(trigger(fixture, 'chat-model-select')!.textContent).toContain(
                'Default (openai/gpt-5)'
            );
        });
    });

    describe('at phone width', () => {
        beforeEach(async () => {
            await page.viewport(390, 844);
        });

        afterEach(async () => {
            await page.viewport(1280, 800);
        });

        it('offers a back control that tells the page to return to the list', async () => {
            const fixture = await setup();
            const backs: unknown[] = [];
            fixture.componentInstance.back.subscribe(() => backs.push(true));

            const back = fixture.nativeElement.querySelector(
                '[data-testid="new-conversation-back"]'
            ) as HTMLElement;
            expect(back.getBoundingClientRect().width).toBeGreaterThan(0);
            expect(
                back.querySelector('button')!.getBoundingClientRect().height
            ).toBeGreaterThanOrEqual(44);
            back.click();

            expect(backs).toEqual([true]);
        });

        it('fits the form into the width with several chosen projects', async () => {
            const fixture = await setup({ idProject: 1 });
            const host = fixture.nativeElement as HTMLElement;
            fixture.componentInstance['form'].controls.idProjects.setValue([1, 2, 3]);
            fixture.detectChanges();
            await fixture.whenStable();
            fixture.detectChanges();

            expect(host.scrollWidth).toBeLessThanOrEqual(host.clientWidth);
            expect(host.querySelector('form')!.getBoundingClientRect().right).toBeLessThanOrEqual(
                390
            );
        });
    });

    describe('at desktop width', () => {
        it('has no back control', async () => {
            await page.viewport(1280, 800);
            const fixture = await setup();

            const back = fixture.nativeElement.querySelector(
                '[data-testid="new-conversation-back"]'
            ) as HTMLElement;
            expect(back.getBoundingClientRect().width).toBe(0);
        });
    });
});

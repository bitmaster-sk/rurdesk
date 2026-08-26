import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { User } from 'src/app/auth/model/user.model';
import { Fixtures } from 'src/testing/fixtures';
import { TablerIconStub } from 'src/testing/stubs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AgentRunApi } from '../../../agent/api/agent-run.api.service';
import { AgentRun } from '../../../agent/model/agent-run.model';
import { AgentStage } from '../../../agent/model/agent-stage.enum';
import { ToastNotificationService } from '../../../core/toast-notification.service';
import { ProjectSkillApi } from '../../../project/api/project-skill.api.service';
import { UiModule } from '../../../ui/ui.module';
import { SkillApi } from '../../api/skill.api.service';
import { AgentOverview } from '../../../agent/model/agent-overview.model';
import { ProjectSkill } from '../../../project/model/project-skill.model';
import { Skill } from '../../model/skill.model';
import { AgentDockComponent, AgentDockKind } from './agent-dock.component';

const BOT: User = Fixtures.bot();
const SKILLS: Skill[] = [
    Fixtures.skill({ idSkill: 1, name: 'Verification rules' }),
    Fixtures.skill({ idSkill: 2, name: 'TDD' })
];
const DEFAULTS: ProjectSkill[] = [{ idProject: 7, idSkill: 1, stage: AgentStage.Implementation }];
const RUN: AgentRun = Fixtures.agentRun();

const WORKLOAD: AgentOverview = {
    idUserBot: 8,
    isBusy: true,
    current: { idIssuePublic: 42, stage: AgentStage.Design },
    queueCount: 3,
    queuedIdsIssuePublic: [43],
    completedToday: 2,
    tokens7d: 412_534,
    avgRunDurationMs7d: 480_000,
    failedAttempts7d: 1
};

abstract class Dom {
    public static chip(root: HTMLElement, idSkill: number, stage: AgentStage): HTMLButtonElement {
        return root.querySelector(`[data-testid="chip-${idSkill}-${stage}"]`) as HTMLButtonElement;
    }

    public static byTestId(root: HTMLElement, testId: string): HTMLElement {
        return root.querySelector(`[data-testid="${testId}"]`) as HTMLElement;
    }

    public static text(root: HTMLElement, testId: string): string {
        return Dom.byTestId(root, testId)?.textContent?.replace(/\s+/g, ' ').trim() ?? '';
    }
}

describe('AgentDockComponent (browser)', () => {
    let skillApi: { load$: ReturnType<typeof vi.fn> };
    let projectSkillApi: { load$: ReturnType<typeof vi.fn> };
    let agentRunApi: { assignAgent$: ReturnType<typeof vi.fn> };
    let toast: { showError: ReturnType<typeof vi.fn> };
    let closeSpy: ReturnType<typeof vi.fn>;

    beforeEach(() => {
        skillApi = { load$: vi.fn().mockReturnValue(of(SKILLS)) };
        projectSkillApi = { load$: vi.fn().mockReturnValue(of(DEFAULTS)) };
        agentRunApi = { assignAgent$: vi.fn().mockReturnValue(of(RUN)) };
        toast = { showError: vi.fn() };
        closeSpy = vi.fn();
    });

    async function setup(kind: AgentDockKind, overview: AgentOverview | null = null) {
        await TestBed.configureTestingModule({
            imports: [UiModule, TranslateModule.forRoot(), TablerIconStub],
            declarations: [AgentDockComponent],
            providers: [
                { provide: SkillApi, useValue: skillApi },
                { provide: ProjectSkillApi, useValue: projectSkillApi },
                { provide: AgentRunApi, useValue: agentRunApi },
                { provide: ToastNotificationService, useValue: toast }
            ]
        }).compileComponents();

        const fixture = TestBed.createComponent(AgentDockComponent);
        fixture.componentRef.setInput('agent', BOT);
        fixture.componentRef.setInput('kind', kind);
        fixture.componentRef.setInput('idProject', 7);
        fixture.componentRef.setInput('idIssuePublic', 42);
        fixture.componentRef.setInput('overview', overview);
        fixture.componentRef.setInput('close', closeSpy);
        fixture.detectChanges();
        return fixture;
    }

    it('renders a chip per skill and stage, pre-checked from the project matrix', async () => {
        const fixture = await setup(AgentDockKind.Skills);
        const root = fixture.nativeElement as HTMLElement;

        expect(root.querySelectorAll('[data-testid^="chip-"]').length).toBe(8);
        expect(Dom.chip(root, 1, AgentStage.Implementation).getAttribute('aria-pressed')).toBe(
            'true'
        );
        expect(Dom.chip(root, 2, AgentStage.Implementation).getAttribute('aria-pressed')).toBe(
            'false'
        );
        expect(Dom.chip(root, 1, AgentStage.Design).getAttribute('aria-pressed')).toBe('false');
    });

    it('assigns with exactly the chips the user left checked', async () => {
        const fixture = await setup(AgentDockKind.Skills);
        const root = fixture.nativeElement as HTMLElement;
        const assigned = vi.fn();
        fixture.componentInstance.assigned.subscribe(assigned);

        Dom.chip(root, 2, AgentStage.Design).click();
        fixture.detectChanges();
        Dom.byTestId(root, 'agent-dock-assign').click();
        fixture.detectChanges();

        expect(agentRunApi.assignAgent$).toHaveBeenCalledWith(7, 42, {
            idUserBot: 8,
            idsSkillByStage: { implementation: [1], design: [2] }
        });
        expect(assigned).toHaveBeenCalledWith(RUN);
        expect(closeSpy).toHaveBeenCalled();
    });

    it('a failed assign toasts and does not emit', async () => {
        agentRunApi.assignAgent$ = vi.fn().mockReturnValue(throwError(() => new Error('nope')));
        const fixture = await setup(AgentDockKind.Skills);
        const assigned = vi.fn();
        fixture.componentInstance.assigned.subscribe(assigned);

        Dom.byTestId(fixture.nativeElement, 'agent-dock-assign').click();
        fixture.detectChanges();

        expect(toast.showError).toHaveBeenCalledWith('AGENT_DOCK.ASSIGN_ERROR');
        expect(assigned).not.toHaveBeenCalled();
        expect(closeSpy).not.toHaveBeenCalled();
    });

    it('info mode renders the workload and names this project queued tasks', async () => {
        const fixture = await setup(AgentDockKind.Info, WORKLOAD);
        const root = fixture.nativeElement as HTMLElement;

        expect(Dom.text(root, 'agent-dock-running')).toContain('#42');
        expect(Dom.text(root, 'agent-dock-queue')).toContain('3');
        expect(Dom.text(root, 'agent-dock-queue')).toContain('#43');
        expect(Dom.text(root, 'agent-dock-tokens')).toBe('412.5k');
        expect(skillApi.load$).not.toHaveBeenCalled();
    });

    it('info mode says so when the bot is running nothing here', async () => {
        const fixture = await setup(AgentDockKind.Info, {
            ...WORKLOAD,
            current: null,
            queuedIdsIssuePublic: []
        });

        expect(Dom.text(fixture.nativeElement, 'agent-dock-running')).toBe('AGENT_DOCK.NONE');
    });
});

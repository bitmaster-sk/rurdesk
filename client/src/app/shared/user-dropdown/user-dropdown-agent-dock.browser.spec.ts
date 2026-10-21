import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormControl, FormsModule, ReactiveFormsModule } from '@angular/forms';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { TranslateModule } from '@ngx-translate/core';
import { of } from 'rxjs';
import { User } from 'src/app/auth/model/user.model';
import { Fixtures } from 'src/testing/fixtures';
import { AvatarStub, TablerIconStub } from 'src/testing/stubs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AgentRunApi } from '../../agent/api/agent-run.api.service';
import { AgentRun } from '../../agent/model/agent-run.model';
import { AgentStage } from '../../agent/model/agent-stage.enum';
import { ToastNotificationService } from '../../core/toast-notification.service';
import { ProjectSkillApi } from '../../project/api/project-skill.api.service';
import { ProjectSkill } from '../../project/model/project-skill.model';
import { UiModule } from '../../ui/ui.module';
import { SkillApi } from '../api/skill.api.service';
import { AgentStagePlanEditorComponent } from '../../agent/components/agent-stage-plan-editor/agent-stage-plan-editor.component';
import { AgentStagePlanFormComponent } from '../../agent/components/agent-stage-plan-form/agent-stage-plan-form.component';
import { AgentDockComponent } from '../components/agent-dock/agent-dock.component';
import { Skill } from '../model/skill.model';
import { UserDropdownComponent } from './user-dropdown.component';

const ADA: User = Fixtures.user({ idUser: 1, name: 'Ada' });
const AGENT: User = Fixtures.agent();
const RUN: AgentRun = Fixtures.agentRun();
const SKILLS: Skill[] = [Fixtures.skill({ idSkill: 1, name: 'TDD' })];
const DEFAULTS: ProjectSkill[] = [{ idProject: 7, idSkill: 1, stage: AgentStage.Implementation }];

@Component({
    standalone: false,
    template: `
        <app-user-dropdown
            [users]="users()"
            [formControl]="ctrl"
            [hasAgentFeatures]="true"
            [idProject]="7"
            [idIssuePublic]="42"
            (agentRunCreated)="lastAssigned = $event"
        />
    `
})
class HostComponent {
    public readonly users = signal<User[]>([ADA, AGENT]);
    public readonly ctrl = new FormControl<number | null>(null);
    public lastAssigned: AgentRun | null = null;
}

abstract class Dom {
    public static byTestId(testId: string): HTMLElement | null {
        return document.querySelector(`[data-testid="${testId}"]`);
    }

    public static option(name: string): HTMLElement {
        return Array.from(document.querySelectorAll<HTMLElement>('.ui-select-panel__option')).find(
            row => row.textContent?.includes(name)
        )!;
    }
}

describe('UserDropdownComponent with the agent dock open (browser)', () => {
    let agentRunApi: {
        loadAgentsOverview$: ReturnType<typeof vi.fn>;
        assignAgent$: ReturnType<typeof vi.fn>;
        loadByIssue$: ReturnType<typeof vi.fn>;
    };

    beforeEach(async () => {
        agentRunApi = {
            loadAgentsOverview$: vi.fn().mockReturnValue(of([])),
            assignAgent$: vi.fn().mockReturnValue(of(RUN)),
            loadByIssue$: vi.fn().mockReturnValue(of(null))
        };

        await TestBed.configureTestingModule({
            imports: [
                UiModule,
                FormsModule,
                ReactiveFormsModule,
                TranslateModule.forRoot(),
                TablerIconStub,
                AvatarStub
            ],
            declarations: [
                HostComponent,
                UserDropdownComponent,
                AgentDockComponent,
                AgentStagePlanEditorComponent,
                AgentStagePlanFormComponent
            ],
            providers: [
                provideNoopAnimations(),
                { provide: AgentRunApi, useValue: agentRunApi },
                { provide: SkillApi, useValue: { load$: () => of(SKILLS) } },
                { provide: ProjectSkillApi, useValue: { load$: () => of(DEFAULTS) } },
                { provide: ToastNotificationService, useValue: { showError: vi.fn() } }
            ]
        }).compileComponents();
    });

    function openSkillsDock() {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.detectChanges();
        (fixture.nativeElement.querySelector('.ui-select-trigger') as HTMLElement).click();
        fixture.detectChanges();
        Dom.byTestId('agent-skills-action')!.click();
        fixture.detectChanges();
        return fixture;
    }

    it('clicking the docked agent name assigns with the dock settings', () => {
        const fixture = openSkillsDock();
        let changeCount = 0;
        fixture.componentInstance.ctrl.valueChanges.subscribe(() => changeCount++);

        Dom.byTestId('agent-dock-short-workflow')!.click();
        fixture.detectChanges();
        Dom.option(AGENT.name).click();
        fixture.detectChanges();

        expect(agentRunApi.assignAgent$).toHaveBeenCalledWith(7, 42, {
            idUserAgent: AGENT.idUser,
            idsSkillByStage: { implementation: [1] },
            isShortWorkflow: true
        });
        expect(changeCount).toBe(0);
        expect(fixture.componentInstance.lastAssigned).toEqual(RUN);
        expect(Dom.byTestId('agent-dock-skills')).toBeNull();
    });

    it('picking another user closes the dock and applies nothing from it', () => {
        const fixture = openSkillsDock();

        Dom.byTestId('agent-dock-short-workflow')!.click();
        fixture.detectChanges();
        Dom.option(ADA.name).click();
        fixture.detectChanges();

        expect(agentRunApi.assignAgent$).not.toHaveBeenCalled();
        expect(fixture.componentInstance.ctrl.value).toBe(ADA.idUser);
        expect(Dom.byTestId('agent-dock-skills')).toBeNull();
    });
});

import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { of } from 'rxjs';
import { AgentRunApi } from 'src/app/agent/api/agent-run.api.service';
import { AgentStage } from 'src/app/agent/model/agent-stage.enum';
import { User } from 'src/app/auth/model/user.model';
import { ProjectSkillApi } from 'src/app/project/api/project-skill.api.service';
import { SkillApi } from 'src/app/shared/api/skill.api.service';
import { describe, expect, it, vi } from 'vitest';
import { StartReleaseAgentReq } from '../../model/release.model';
import { ReleaseAgentFormComponent } from './release-agent-form.component';

const AGENTS: User[] = [
    { idUser: 4, name: 'Bot', email: 'bot@x', colorAvatarBg: '#000', isAgent: true },
    { idUser: 6, name: 'Writer', email: 'w@x', colorAvatarBg: '#000', isAgent: true }
];

async function setup() {
    await TestBed.configureTestingModule({
        imports: [TranslateModule.forRoot()],
        declarations: [ReleaseAgentFormComponent],
        providers: [
            { provide: SkillApi, useValue: { load$: () => of([]) } },
            {
                provide: ProjectSkillApi,
                useValue: {
                    load$: () =>
                        of([
                            { idSkill: 1, stage: AgentStage.ReleaseNotes },
                            { idSkill: 2, stage: AgentStage.Implementation }
                        ])
                }
            },
            {
                provide: AgentRunApi,
                useValue: {
                    loadAgentsOverview$: () =>
                        of([
                            {
                                idUserAgent: 6,
                                models: ['opus', 'sonnet'],
                                modelDefault: 'sonnet',
                                maxTurnsPlan: 40
                            }
                        ])
                }
            }
        ]
    })
        .overrideComponent(ReleaseAgentFormComponent, { set: { template: '' } })
        .compileComponents();
    const fixture = TestBed.createComponent(ReleaseAgentFormComponent);
    fixture.componentRef.setInput('idProject', 3);
    fixture.componentRef.setInput('agents', AGENTS);
    fixture.detectChanges();
    const started = vi.fn<(req: StartReleaseAgentReq) => void>();
    fixture.componentInstance.started.subscribe(started);
    return { form: fixture.componentInstance, started };
}

describe('ReleaseAgentFormComponent', () => {
    it('starts the first agent with the release notes skills of the project', async () => {
        const { form, started } = await setup();

        form['onStart']();

        expect(started).toHaveBeenCalledWith({
            idUserAgent: 4,
            idsSkillByStage: { [AgentStage.ReleaseNotes]: [1] },
            isShortWorkflow: false
        });
    });

    it('sends the model and turns picked for the chosen agent', async () => {
        const { form, started } = await setup();

        form['idUserAgent'].setValue(6);
        form['onModelPicked']('opus');
        form['onMaxTurns'](12);
        form['onSkillToggled'](1);
        form['onStart']();

        expect(form['row']().defaultMaxTurns).toBe(40);
        expect(started).toHaveBeenCalledWith({
            idUserAgent: 6,
            idsSkillByStage: { [AgentStage.ReleaseNotes]: [] },
            isShortWorkflow: false,
            modelByStage: { [AgentStage.ReleaseNotes]: 'opus' },
            maxTurnsByStage: { [AgentStage.ReleaseNotes]: 12 }
        });
    });

    it('fills in the default model of the gateway of the chosen agent', async () => {
        const { form, started } = await setup();

        expect(form['row']().model).toBeNull();
        form['idUserAgent'].setValue(6);
        expect(form['row']().model).toBe('sonnet');
        form['onStart']();

        expect(started).toHaveBeenCalledWith(
            expect.objectContaining({ modelByStage: { [AgentStage.ReleaseNotes]: 'sonnet' } })
        );
    });
});

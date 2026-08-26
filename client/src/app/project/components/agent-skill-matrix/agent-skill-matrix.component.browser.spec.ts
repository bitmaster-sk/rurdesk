import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { Fixtures } from 'src/testing/fixtures';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AgentStage } from '../../../agent/model/agent-stage.enum';
import { ToastNotificationService } from '../../../core/toast-notification.service';
import { SkillApi } from '../../../shared/api/skill.api.service';
import { Skill } from '../../../shared/model/skill.model';
import { ProjectSkill } from '../../model/project-skill.model';
import { UiSaveState } from '../../../ui/components/save-status/save-status-chip.component';
import { ProjectSkillApi } from '../../api/project-skill.api.service';
import { Project } from '../../model/project.model';
import { AgentSkillMatrixComponent } from './agent-skill-matrix.component';

const SKILLS: Skill[] = [
    Fixtures.skill({ idSkill: 1, name: 'Verification rules' }),
    Fixtures.skill({ idSkill: 2, name: 'House style', isBuiltin: false })
];
const SAVED: ProjectSkill[] = [{ idProject: 7, idSkill: 1, stage: AgentStage.Implementation }];
const PROJECT: Project = { idProject: 7, name: 'p', color: '#123456' };

abstract class Dom {
    public static cell(fixture: { nativeElement: HTMLElement }, key: string): HTMLElement {
        return fixture.nativeElement.querySelector(`[data-cell="${key}"]`) as HTMLElement;
    }
}

describe('AgentSkillMatrixComponent (browser)', () => {
    let skillApi: { load$: ReturnType<typeof vi.fn> };
    let projectSkillApi: {
        load$: ReturnType<typeof vi.fn>;
        replace$: ReturnType<typeof vi.fn>;
    };
    let toast: { showError: ReturnType<typeof vi.fn> };

    beforeEach(() => {
        skillApi = { load$: vi.fn().mockReturnValue(of(SKILLS)) };
        projectSkillApi = {
            load$: vi.fn().mockReturnValue(of(SAVED)),
            replace$: vi.fn().mockReturnValue(of(SAVED))
        };
        toast = { showError: vi.fn() };
    });

    async function setup() {
        await TestBed.configureTestingModule({
            imports: [TranslateModule.forRoot()],
            declarations: [AgentSkillMatrixComponent],
            providers: [
                { provide: SkillApi, useValue: skillApi },
                { provide: ProjectSkillApi, useValue: projectSkillApi },
                { provide: ToastNotificationService, useValue: toast }
            ]
        })
            .overrideComponent(AgentSkillMatrixComponent, {
                set: {
                    template: `
                        @for (skill of skills(); track skill.idSkill) {
                            <div class="row">
                                <span class="name">{{ skill.name }}</span>
                                @for (stage of stages; track stage) {
                                    <button
                                        class="cell"
                                        [class.on]="isEnabled(skill.idSkill, stage)"
                                        [attr.data-cell]="skill.idSkill + ':' + stage"
                                        (click)="onToggle(skill.idSkill, stage)"
                                    ></button>
                                }
                                <span class="status">{{ rowSaveStatus(skill.idSkill) }}</span>
                            </div>
                        }
                    `
                }
            })
            .compileComponents();

        const fixture = TestBed.createComponent(AgentSkillMatrixComponent);
        fixture.componentRef.setInput('project', PROJECT);
        fixture.detectChanges();
        return fixture;
    }

    it('renders a row per skill with one cell per non-pickup stage', async () => {
        const fixture = await setup();

        expect(fixture.nativeElement.querySelectorAll('.row').length).toBe(2);
        expect(
            fixture.nativeElement.querySelectorAll('.row')[0].querySelectorAll('.cell').length
        ).toBe(4);
    });

    it('pre-checks the cells the project already has enabled', async () => {
        const fixture = await setup();

        expect(Dom.cell(fixture, '1:implementation').classList).toContain('on');
        expect(Dom.cell(fixture, '1:design').classList).not.toContain('on');
        expect(Dom.cell(fixture, '2:implementation').classList).not.toContain('on');
    });

    it('toggling a cell sends the full matrix and flips the row status to saved', async () => {
        const fixture = await setup();

        Dom.cell(fixture, '2:design').click();
        fixture.detectChanges();

        expect(projectSkillApi.replace$).toHaveBeenCalledTimes(1);
        const [idProject, entries] = projectSkillApi.replace$.mock.calls[0];
        expect(idProject).toBe(7);
        expect(entries).toEqual(
            expect.arrayContaining([
                { idSkill: 1, stage: 'implementation' },
                { idSkill: 2, stage: 'design' }
            ])
        );
        expect(fixture.nativeElement.querySelectorAll('.status')[1].textContent).toBe(
            UiSaveState.Saved
        );
    });

    it('unchecking removes the cell from the saved matrix', async () => {
        const fixture = await setup();

        Dom.cell(fixture, '1:implementation').click();
        fixture.detectChanges();

        expect(projectSkillApi.replace$).toHaveBeenCalledWith(7, []);
        expect(Dom.cell(fixture, '1:implementation').classList).not.toContain('on');
    });

    it('a failed save flips the row to error and toasts', async () => {
        projectSkillApi.replace$ = vi.fn().mockReturnValue(throwError(() => new Error('nope')));
        const fixture = await setup();

        Dom.cell(fixture, '2:design').click();
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelectorAll('.status')[1].textContent).toBe(
            UiSaveState.Error
        );
        expect(toast.showError).toHaveBeenCalledWith('AGENT.SKILLS.SAVE_ERROR');
    });
});

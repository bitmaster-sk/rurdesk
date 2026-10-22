import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { RouterModule, provideRouter } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { Fixtures } from 'src/testing/fixtures';
import { TablerIconStub } from 'src/testing/stubs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { UiModule } from '../../../ui/ui.module';
import { AgentStageCardConverter } from '../../converter/agent-stage-card.converter';
import { AgentStageCardView } from '../../entity/agent-stage-card.entity';
import { AgentStagePlanEditorRow } from '../../entity/agent-stage-plan-editor.entity';
import { RunWikiStage } from '../../entity/run-wiki-context.entity';
import { AgentStageProgress } from '../../model/agent-run.model';
import { AgentStage } from '../../model/agent-stage.enum';
import { AgentStagePlanFormComponent } from '../agent-stage-plan-form/agent-stage-plan-form.component';
import { RunWikiStageComponent } from '../run-wiki-stage/run-wiki-stage.component';
import { AgentStageCardComponent } from './agent-stage-card.component';

const SKILLS = [
    Fixtures.skill({ idSkill: 1, name: 'go-backend' }),
    Fixtures.skill({ idSkill: 2, name: 'angular-client' })
];

const PLAN_ROW: AgentStagePlanEditorRow = {
    stage: AgentStage.Implementation,
    idsSkill: [1],
    model: null,
    maxTurns: null,
    defaultMaxTurns: 500,
    lock: null
};

const WIKI: RunWikiStage = {
    stage: 'design',
    prompt: [
        { key: 1, title: 'Commit messages', versionNo: 1, tokens: 0, link: null, isAlways: true }
    ],
    index: [],
    calls: [],
    promptTokens: 0,
    indexTokens: 0,
    openedTokens: 0
};

function view(progress: Partial<AgentStageProgress>, isEditable = true): AgentStageCardView {
    const [card] = AgentStageCardConverter.toViews({
        stages: [Fixtures.stageProgress(progress)],
        plan: [
            {
                name: progress.stage ?? 'design',
                idsSkill: [1, 2],
                model: null,
                maxTurns: null,
                dispatched: false
            }
        ],
        runModel: 'openai/kimi',
        defaultMaxTurns: () => 40,
        skillNames: new Map(SKILLS.map(skill => [skill.idSkill, skill.name])),
        agentNames: new Map(),
        isEditable
    });
    return card;
}

describe('AgentStageCardComponent (browser)', () => {
    let fixture: ComponentFixture<AgentStageCardComponent>;
    let root: HTMLElement;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                UiModule,
                FormsModule,
                TranslateModule.forRoot(),
                TablerIconStub,
                RouterModule
            ],
            declarations: [
                AgentStageCardComponent,
                AgentStagePlanFormComponent,
                RunWikiStageComponent
            ],
            providers: [provideRouter([])]
        }).compileComponents();
        fixture = TestBed.createComponent(AgentStageCardComponent);
        root = fixture.nativeElement as HTMLElement;
        fixture.componentRef.setInput('skills', SKILLS);
    });

    function render(card: AgentStageCardView, inputs: Record<string, unknown> = {}): void {
        fixture.componentRef.setInput('view', card);
        Object.entries(inputs).forEach(([name, value]) =>
            fixture.componentRef.setInput(name, value)
        );
        fixture.detectChanges();
    }

    function byTestId(testId: string): HTMLElement | null {
        return root.querySelector(`[data-testid="${testId}"]`);
    }

    function text(testId: string): string {
        return byTestId(testId)?.textContent?.replace(/\s+/g, ' ').trim() ?? '';
    }

    function open(stage: string): void {
        byTestId(`stage-card-toggle-${stage}`)!.click();
        fixture.detectChanges();
    }

    it('lets the user change the plan of the next stage and saves each change', () => {
        render(view({ stage: AgentStage.Implementation }), { planRow: PLAN_ROW });
        const toggled = vi.fn();
        fixture.componentInstance.skillToggled.subscribe(toggled);

        byTestId(`chip-2-${AgentStage.Implementation}`)!.click();

        expect(byTestId(`stage-plan-form-${AgentStage.Implementation}`)).not.toBeNull();
        expect(toggled).toHaveBeenCalledWith({ stage: AgentStage.Implementation, idSkill: 2 });
    });

    it('shows a stage that ran with its numbers, what it ran with and its wiki, but no editor', () => {
        render(
            view({
                stage: 'design',
                status: 'awaiting_approval',
                attempts: 1,
                toolCallsCount: 15,
                tokensUsed: 118283,
                model: 'openai/kimi',
                maxTurns: 40
            }),
            { planRow: { ...PLAN_ROW, stage: AgentStage.Design }, wikiStage: WIKI }
        );

        expect(byTestId('stage-plan-form-design')).toBeNull();
        expect(text('stage-card-usage-design')).toBe(
            '1 AGENT.STAGE_CARD.ATTEMPTS.SINGULAR · 15 AGENT.STAGE_CARD.TOOL_CALLS.PLURAL · 118,283 AGENT.STAGE_CARD.TOKENS.PLURAL'
        );
        expect(
            Array.from(byTestId('stage-card-usage-design')!.querySelectorAll('b')).map(
                bold => bold.textContent
            )
        ).toEqual(['1', '15', '118,283']);
        expect(text('stage-card-ran-model')).toBe('openai/kimi');
        expect(
            Array.from(byTestId('stage-card-ran-skills')!.querySelectorAll('span')).map(pill =>
                pill.textContent?.trim()
            )
        ).toEqual(['go-backend', 'angular-client']);
        expect(text('stage-card-ran-turns')).toBe('40');
        expect(text('run-wiki-stage')).toContain('Commit messages');
    });

    it('keeps a finished stage closed until the user opens it', () => {
        render(view({ stage: 'brainstorming', status: 'done', attempts: 1 }));
        expect(byTestId('stage-card-body-brainstorming')).toBeNull();

        open('brainstorming');

        expect(byTestId('stage-card-usage-brainstorming')).not.toBeNull();
    });

    it('shows the plan of a stage that has not started as tags in its header', () => {
        render(view({ stage: AgentStage.Implementation }), { planRow: PLAN_ROW });

        expect(text(`stage-card-skills-${AgentStage.Implementation}`)).toBe(
            'AGENT.STAGE_CARD.SKILLS_TAG.PLURAL'
        );
    });

    it('a skipped stage cannot be opened', () => {
        render(view({ stage: 'design', status: 'skipped' }));

        expect((byTestId('stage-card-toggle-design') as HTMLButtonElement).disabled).toBe(true);
        expect(byTestId('stage-card-body-design')).toBeNull();
    });
});

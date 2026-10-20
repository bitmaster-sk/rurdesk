import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { Fixtures } from 'src/testing/fixtures';
import { TablerIconStub } from 'src/testing/stubs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { UiModule } from '../../../ui/ui.module';
import {
    AgentStagePlanEditorLock,
    AgentStagePlanEditorRow
} from '../../entity/agent-stage-plan-editor.entity';
import { AgentStage } from '../../model/agent-stage.enum';
import { AgentStagePlanEditorComponent } from './agent-stage-plan-editor.component';

const SKILLS = [
    Fixtures.skill({ idSkill: 1, name: 'Verification rules' }),
    Fixtures.skill({ idSkill: 2, name: 'TDD' })
];

function row(
    stage: AgentStage,
    overrides: Partial<AgentStagePlanEditorRow> = {}
): AgentStagePlanEditorRow {
    return {
        stage,
        idsSkill: [],
        model: null,
        maxTurns: null,
        defaultMaxTurns: 250,
        lock: null,
        ...overrides
    };
}

abstract class Dom {
    public static byTestId(root: HTMLElement, testId: string): HTMLElement | null {
        return root.querySelector(`[data-testid="${testId}"]`);
    }

    public static text(root: HTMLElement, testId: string): string {
        return Dom.byTestId(root, testId)?.textContent?.replace(/\s+/g, ' ').trim() ?? '';
    }
}

describe('AgentStagePlanEditorComponent (browser)', () => {
    let fixture: ComponentFixture<AgentStagePlanEditorComponent>;
    let root: HTMLElement;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [UiModule, FormsModule, TranslateModule.forRoot(), TablerIconStub],
            declarations: [AgentStagePlanEditorComponent]
        }).compileComponents();
        fixture = TestBed.createComponent(AgentStagePlanEditorComponent);
        root = fixture.nativeElement as HTMLElement;
    });

    function render(rows: AgentStagePlanEditorRow[], models: string[] = []): void {
        fixture.componentRef.setInput('rows', rows);
        fixture.componentRef.setInput('skills', SKILLS);
        fixture.componentRef.setInput('models', models);
        fixture.detectChanges();
    }

    it('a closed stage summarises its skills, model and turns', () => {
        render([row(AgentStage.Design, { idsSkill: [2], maxTurns: 90 })], ['openai/a']);

        expect(Dom.byTestId(root, `chip-2-${AgentStage.Design}`)).toBeNull();
        expect(Dom.text(root, `stage-plan-summary-skills-${AgentStage.Design}`)).toBe('TDD');
        expect(Dom.text(root, `stage-plan-summary-model-${AgentStage.Design}`)).toBe(
            'AGENT_STAGE_PLAN_EDITOR.MODEL_INHERIT · AGENT_STAGE_PLAN_EDITOR.TURNS'
        );
    });

    it('an opened stage emits the skill the user toggled', () => {
        render([row(AgentStage.Design, { idsSkill: [2] })]);
        const toggled = vi.fn();
        fixture.componentInstance.skillToggled.subscribe(toggled);

        Dom.byTestId(root, `stage-plan-toggle-${AgentStage.Design}`)!.click();
        fixture.detectChanges();
        const chip = Dom.byTestId(root, `chip-1-${AgentStage.Design}`)!;
        chip.click();

        expect(
            Dom.byTestId(root, `chip-2-${AgentStage.Design}`)!.getAttribute('aria-pressed')
        ).toBe('true');
        expect(toggled).toHaveBeenCalledWith({ stage: AgentStage.Design, idSkill: 1 });
    });

    it('a changed turn limit is emitted, and an emptied one falls back to the default', () => {
        render([row(AgentStage.Implementation, { maxTurns: 90, defaultMaxTurns: 500 })]);
        const changed = vi.fn();
        fixture.componentInstance.maxTurnsChanged.subscribe(changed);
        Dom.byTestId(root, `stage-plan-toggle-${AgentStage.Implementation}`)!.click();
        fixture.detectChanges();
        const turns = Dom.byTestId(
            root,
            `stage-turns-${AgentStage.Implementation}`
        ) as HTMLInputElement;

        expect(turns.placeholder).toBe('500');
        turns.value = '';
        turns.dispatchEvent(new Event('change'));

        expect(changed).toHaveBeenCalledWith({ stage: AgentStage.Implementation, maxTurns: null });
    });

    it('a skipped stage shows only its badge', () => {
        render([row(AgentStage.Design, { lock: AgentStagePlanEditorLock.Skipped })]);

        expect(Dom.text(root, `stage-plan-locked-${AgentStage.Design}`)).toBe(
            'AGENT_STAGE_PLAN_EDITOR.SKIPPED'
        );
        expect(Dom.byTestId(root, `stage-plan-toggle-${AgentStage.Design}`)).toBeNull();
        expect(Dom.byTestId(root, `stage-plan-summary-${AgentStage.Design}`)).toBeNull();
    });

    it('a started stage keeps its summary but cannot be opened', () => {
        render([row(AgentStage.Design, { idsSkill: [1], lock: AgentStagePlanEditorLock.Started })]);

        expect(Dom.text(root, `stage-plan-locked-${AgentStage.Design}`)).toBe(
            'AGENT_STAGE_PLAN_EDITOR.STARTED'
        );
        expect(Dom.byTestId(root, `stage-plan-toggle-${AgentStage.Design}`)).toBeNull();
        expect(Dom.text(root, `stage-plan-summary-skills-${AgentStage.Design}`)).toBe(
            'Verification rules'
        );
    });
});

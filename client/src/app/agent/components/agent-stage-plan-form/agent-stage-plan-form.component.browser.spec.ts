import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { Fixtures } from 'src/testing/fixtures';
import { TablerIconStub } from 'src/testing/stubs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { UiModule } from '../../../ui/ui.module';
import { AgentStagePlanEditorRow } from '../../entity/agent-stage-plan-editor.entity';
import { AgentStage } from '../../model/agent-stage.enum';
import { AgentStagePlanFormComponent } from './agent-stage-plan-form.component';
import { AgentEffortPickerComponent } from '../agent-effort-picker/agent-effort-picker.component';
import { AgentEffortHintComponent } from '../agent-effort-hint/agent-effort-hint.component';
import { AgentEffortLabelComponent } from '../agent-effort-label/agent-effort-label.component';
import { AgentEffortMeterComponent } from '../agent-effort-meter/agent-effort-meter.component';
import { AgentEffort } from '../../model/agent-effort.enum';

const ROW: AgentStagePlanEditorRow = {
    stage: AgentStage.Design,
    idsSkill: [2],
    model: null,
    maxTurns: null,
    defaultMaxTurns: 250,
    effort: null,
    inheritedEffort: null,
    isThinkingSupported: true,
    lock: null
};

describe('AgentStagePlanFormComponent (browser)', () => {
    let fixture: ComponentFixture<AgentStagePlanFormComponent>;
    let root: HTMLElement;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [UiModule, FormsModule, TranslateModule.forRoot(), TablerIconStub],
            declarations: [
                AgentStagePlanFormComponent,
                AgentEffortPickerComponent,
                AgentEffortHintComponent,
                AgentEffortLabelComponent,
                AgentEffortMeterComponent
            ]
        }).compileComponents();
        fixture = TestBed.createComponent(AgentStagePlanFormComponent);
        root = fixture.nativeElement as HTMLElement;
        fixture.componentRef.setInput('row', ROW);
        fixture.componentRef.setInput('skills', [
            Fixtures.skill({ idSkill: 1, name: 'Verification rules' }),
            Fixtures.skill({ idSkill: 2, name: 'TDD' })
        ]);
    });

    it('marks the chosen skills and emits the one the user toggled', () => {
        fixture.detectChanges();
        const toggled = vi.fn();
        fixture.componentInstance.skillToggled.subscribe(toggled);

        (root.querySelector('[data-testid="chip-1-design"]') as HTMLElement).click();

        expect(
            root.querySelector('[data-testid="chip-2-design"]')!.getAttribute('aria-pressed')
        ).toBe('true');
        expect(toggled).toHaveBeenCalledWith({ stage: AgentStage.Design, idSkill: 1 });
    });

    it('emits a turn limit typed into the field', () => {
        fixture.detectChanges();
        const changed = vi.fn();
        fixture.componentInstance.maxTurnsChanged.subscribe(changed);
        const turns = root.querySelector('[data-testid="stage-turns-design"]') as HTMLInputElement;

        turns.value = '80';
        turns.dispatchEvent(new Event('change'));

        expect(changed).toHaveBeenCalledWith({ stage: AgentStage.Design, maxTurns: 80 });
    });

    it('explains the run defaults only when asked to', () => {
        fixture.detectChanges();
        expect(root.textContent).not.toContain('AGENT_STAGE_PLAN_EDITOR.DEFAULTS_HINT');

        fixture.componentRef.setInput('isDefaultsHintShown', true);
        fixture.detectChanges();

        expect(root.textContent).toContain('AGENT_STAGE_PLAN_EDITOR.DEFAULTS_HINT');
    });

    it('renders compact fields by default and form-sized ones on request', () => {
        fixture.componentRef.setInput('models', ['opus']);
        fixture.detectChanges();
        const turns = root.querySelector('[data-testid="stage-turns-design"]') as HTMLElement;
        expect(turns.classList).toContain('ui-input--sm');
        expect(root.querySelector('.text-xs')).not.toBeNull();
        expect(root.querySelector('.stage-plan-form')?.classList).toContain('gap-2');

        fixture.componentRef.setInput('size', 'medium');
        fixture.detectChanges();

        expect(turns.classList).not.toContain('ui-input--sm');
        expect(root.querySelector('.text-xs')).toBeNull();
        expect(root.querySelector('.stage-plan-form')?.classList).toContain('gap-4');
        expect(root.querySelector('.text-sm')?.textContent).toContain(
            'AGENT_STAGE_PLAN_EDITOR.SKILLS'
        );
    });

    it('shows the placeholder it is given for the model', () => {
        fixture.componentRef.setInput('models', ['opus']);
        fixture.componentRef.setInput('modelPlaceholder', 'RELEASE.AGENT.MODEL_DEFAULT');
        fixture.detectChanges();

        expect(root.querySelector('[data-testid="stage-model-design"]')?.textContent).toContain(
            'RELEASE.AGENT.MODEL_DEFAULT'
        );
    });

    it('emits the thinking effort picked for the stage', () => {
        fixture.detectChanges();
        const picked = vi.fn();
        fixture.componentInstance.effortPicked.subscribe(picked);

        (root.querySelector('[data-testid="stage-effort-design-high"]') as HTMLElement).click();

        expect(picked).toHaveBeenCalledWith({ stage: AgentStage.Design, effort: AgentEffort.High });
    });

    it('offers no effort for a model without thinking', () => {
        fixture.componentRef.setInput('row', { ...ROW, isThinkingSupported: false });
        fixture.detectChanges();

        expect(root.querySelector('app-agent-effort-picker')).toBeNull();
    });
});

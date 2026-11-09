import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormGroup, ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { describe, expect, it } from 'vitest';
import { UiModule } from 'src/app/ui/ui.module';
import { AgentRunDefaultsFieldsComponent } from './agent-run-defaults-fields.component';
import { AgentEffortLabelComponent } from 'src/app/agent/components/agent-effort-label/agent-effort-label.component';
import { AgentEffortMeterComponent } from 'src/app/agent/components/agent-effort-meter/agent-effort-meter.component';
import { AgentEffort } from 'src/app/agent/model/agent-effort.enum';
import { AgentStage } from 'src/app/agent/model/agent-stage.enum';
import { AgentStageOptionsByStage } from 'src/app/agent/model/agent-stage-options.model';
import { Fixtures } from 'src/testing/fixtures';
import { AgentStageDefaultsFormBuilder } from '../../util/agent-stage-defaults-form';

@Component({
    standalone: false,
    template: `
        <form [formGroup]="form">
            <app-agent-run-defaults-fields idPrefix="t" [inherited]="inherited()" />
        </form>
    `
})
class HostComponent {
    public readonly form = new FormGroup({
        stageDefaults: AgentStageDefaultsFormBuilder.build(false)
    });
    public readonly inherited = signal<AgentStageOptionsByStage | null>(null);
}

describe('AgentRunDefaultsFieldsComponent (browser)', () => {
    async function render(inherited: AgentStageOptionsByStage | null) {
        await TestBed.configureTestingModule({
            imports: [ReactiveFormsModule, TranslateModule.forRoot(), UiModule],
            declarations: [
                HostComponent,
                AgentRunDefaultsFieldsComponent,
                AgentEffortLabelComponent,
                AgentEffortMeterComponent
            ]
        }).compileComponents();
        const fixture = TestBed.createComponent(HostComponent);
        fixture.componentInstance.inherited.set(inherited);
        AgentStageDefaultsFormBuilder.patch(fixture.componentInstance.form.controls.stageDefaults, {
            [AgentStage.Brainstorming]: { maxTurns: 120, effort: AgentEffort.High }
        });
        fixture.detectChanges();
        return { fixture, root: fixture.nativeElement as HTMLElement };
    }

    function turns(root: HTMLElement, stage: AgentStage): HTMLInputElement {
        return root.querySelector(`[data-testid="t-${stage}-max-turns"]`) as HTMLInputElement;
    }

    function effort(root: HTMLElement, stage: AgentStage): string {
        return root.querySelector(`[data-testid="t-${stage}-effort"]`)?.textContent ?? '';
    }

    it('offers a row for every stage an agent runs, grouped by run kind', async () => {
        const { root } = await render(null);

        expect(root.querySelector('legend')?.textContent).toContain('AGENT_API_KEY.STAGE_DEFAULTS');
        expect(root.textContent).toContain('AGENT.RUN_KIND.ISSUE');
        expect(root.textContent).toContain('AGENT.RUN_KIND.RELEASE_NOTES');
        expect(
            [
                AgentStage.Brainstorming,
                AgentStage.Design,
                AgentStage.ImplementationPlan,
                AgentStage.Implementation,
                AgentStage.ReleaseNotes
            ].every(stage => turns(root, stage))
        ).toBe(true);
        expect(turns(root, AgentStage.Pickup)).toBeNull();
    });

    it('shows the values of the parent form', async () => {
        const { root } = await render(null);

        expect(turns(root, AgentStage.Brainstorming).value).toBe('120');
        expect(effort(root, AgentStage.Brainstorming)).toContain('AGENT_EFFORT.LEVEL.HIGH');
        expect(turns(root, AgentStage.Design).value).toBe('');
        expect(effort(root, AgentStage.Design)).toContain('SETTINGS.AGENT_EFFORT_MODEL_DEFAULT');
    });

    it('writes a typed limit into the stage of the parent form', async () => {
        const { fixture, root } = await render(null);
        const input = turns(root, AgentStage.ReleaseNotes);

        input.value = '60';
        input.dispatchEvent(new Event('input'));

        expect(
            fixture.componentInstance.form.controls.stageDefaults.controls[AgentStage.ReleaseNotes]
                .controls.maxTurns.value
        ).toBe(60);
    });

    it('shows what an empty stage inherits from the app settings', async () => {
        const { root } = await render(
            Fixtures.appAgentStageDefaults({
                [AgentStage.Design]: { maxTurns: 150, effort: AgentEffort.Max }
            })
        );

        expect(turns(root, AgentStage.Design).placeholder).toBe('AGENT_API_KEY.FROM_SETTINGS');
        expect(effort(root, AgentStage.Design)).toContain('AGENT_API_KEY.FROM_SETTINGS');
        expect(effort(root, AgentStage.Implementation)).toContain(
            'SETTINGS.AGENT_EFFORT_MODEL_DEFAULT'
        );
        expect(root.textContent).toContain('AGENT_API_KEY.STAGE_DEFAULTS_HINT');
    });

    it('leaves the turn limit without a placeholder where nothing is inherited', async () => {
        const { root } = await render(null);

        expect(turns(root, AgentStage.Design).placeholder).toBe('');
        expect(root.textContent).not.toContain('AGENT_API_KEY.STAGE_DEFAULTS_HINT');
    });
});

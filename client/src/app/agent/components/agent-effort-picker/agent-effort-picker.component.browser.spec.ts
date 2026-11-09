import { ComponentFixture, TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { TablerIconStub, UiTooltipStub } from 'src/testing/stubs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AgentEffortOrigin } from '../../entity/agent-effort.entity';
import { AgentEffort } from '../../model/agent-effort.enum';
import { AgentEffortHintComponent } from '../agent-effort-hint/agent-effort-hint.component';
import { AgentEffortLabelComponent } from '../agent-effort-label/agent-effort-label.component';
import { AgentEffortMeterComponent } from '../agent-effort-meter/agent-effort-meter.component';
import { AgentEffortPickerComponent } from './agent-effort-picker.component';

describe('AgentEffortPickerComponent (browser)', () => {
    let fixture: ComponentFixture<AgentEffortPickerComponent>;
    let root: HTMLElement;
    let picked: ReturnType<typeof vi.fn<(effort: AgentEffort | null) => void>>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [TranslateModule.forRoot(), TablerIconStub, UiTooltipStub],
            declarations: [
                AgentEffortPickerComponent,
                AgentEffortHintComponent,
                AgentEffortLabelComponent,
                AgentEffortMeterComponent
            ]
        }).compileComponents();
        fixture = TestBed.createComponent(AgentEffortPickerComponent);
        root = fixture.nativeElement as HTMLElement;
        picked = vi.fn<(effort: AgentEffort | null) => void>();
        fixture.componentInstance.effortPicked.subscribe(picked);
    });

    function level(effort: AgentEffort): HTMLButtonElement {
        return root.querySelector(`[data-testid="effort-${effort}"]`) as HTMLButtonElement;
    }

    it('offers the four levels in order', () => {
        fixture.detectChanges();

        const labels = [...root.querySelectorAll('.effort-picker__level')].map(button =>
            button.textContent?.trim()
        );
        expect(labels).toEqual([
            'AGENT_EFFORT.LEVEL.LOW',
            'AGENT_EFFORT.LEVEL.MEDIUM',
            'AGENT_EFFORT.LEVEL.HIGH',
            'AGENT_EFFORT.LEVEL.MAX'
        ]);
    });

    it('emits the level the user clicked', () => {
        fixture.detectChanges();

        level(AgentEffort.High).click();

        expect(picked).toHaveBeenCalledWith(AgentEffort.High);
    });

    it('clicking the chosen level again goes back to the default', () => {
        fixture.componentRef.setInput('effort', AgentEffort.Max);
        fixture.detectChanges();

        expect(level(AgentEffort.Max).getAttribute('aria-pressed')).toBe('true');
        level(AgentEffort.Max).click();

        expect(picked).toHaveBeenCalledWith(null);
    });

    it('offers the way back to the default only once a level is chosen', () => {
        fixture.detectChanges();
        expect(root.querySelector('[data-testid="effort-reset"]')).toBeNull();

        fixture.componentRef.setInput('effort', AgentEffort.Low);
        fixture.detectChanges();
        (root.querySelector('[data-testid="effort-reset"]') as HTMLElement).click();

        expect(picked).toHaveBeenCalledWith(null);
    });

    it('outlines the inherited level and names where it comes from', () => {
        fixture.componentRef.setInput('inherited', {
            effort: AgentEffort.High,
            origin: AgentEffortOrigin.Agent,
            agentName: 'Kimi'
        });
        fixture.detectChanges();

        expect(level(AgentEffort.High).classList).toContain('effort-picker__level--inherited');
        expect(level(AgentEffort.High).getAttribute('aria-pressed')).toBe('false');
        expect(root.querySelector('[data-testid="effort-hint"]')?.textContent).toContain(
            'AGENT_EFFORT.DEFAULT_FROM_AGENT'
        );
    });

    it('stops outlining the inherited level once the stage overrides it', () => {
        fixture.componentRef.setInput('inherited', {
            effort: AgentEffort.High,
            origin: AgentEffortOrigin.Run,
            agentName: null
        });
        fixture.componentRef.setInput('effort', AgentEffort.Low);
        fixture.detectChanges();

        expect(root.querySelector('.effort-picker__level--inherited')).toBeNull();
    });

    it('explains the default behind a help icon', () => {
        fixture.detectChanges();
        expect(root.querySelector('[data-testid="effort-help"]')).toBeNull();

        fixture.componentRef.setInput('hint', 'Default: from the model');
        fixture.detectChanges();

        expect(root.querySelector('[data-testid="effort-help"]')?.getAttribute('aria-label')).toBe(
            'Default: from the model'
        );
    });
});

describe('AgentEffortHintComponent (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [TranslateModule.forRoot()],
            declarations: [AgentEffortHintComponent]
        }).compileComponents();
    });

    function hint(origin: AgentEffortOrigin, effort: AgentEffort | null): string {
        const fixture = TestBed.createComponent(AgentEffortHintComponent);
        fixture.componentRef.setInput('inherited', { effort, origin, agentName: 'Kimi' });
        fixture.detectChanges();
        return (fixture.nativeElement as HTMLElement).textContent?.trim() ?? '';
    }

    it('names the source of the default', () => {
        expect(hint(AgentEffortOrigin.Run, AgentEffort.Max)).toBe('AGENT_EFFORT.DEFAULT_FROM_RUN');
        expect(hint(AgentEffortOrigin.Agent, AgentEffort.High)).toBe(
            'AGENT_EFFORT.DEFAULT_FROM_AGENT'
        );
        expect(hint(AgentEffortOrigin.Settings, AgentEffort.Medium)).toBe(
            'AGENT_EFFORT.DEFAULT_FROM_SETTINGS'
        );
    });

    it('leaves it to the model when nothing sets an effort', () => {
        expect(hint(AgentEffortOrigin.Model, null)).toBe('AGENT_EFFORT.DEFAULT_FROM_MODEL');
    });
});

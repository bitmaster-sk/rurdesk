import { ComponentFixture, TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { TablerIconStub } from 'src/testing/stubs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { UiModule } from '../../../ui/ui.module';
import { RunStats } from '../../model/agent-run.model';
import { AgentRunSummaryComponent } from './agent-run-summary.component';

const STATS: RunStats = {
    totalTokensUsed: 206493,
    totalDurationMs: 99000,
    totalToolCallsCount: 26,
    attemptsPerStage: { design: 1 },
    failedAttempts: 0,
    tokensByModel: { 'openai/kimi': 206493 }
};

describe('AgentRunSummaryComponent (browser)', () => {
    let fixture: ComponentFixture<AgentRunSummaryComponent>;
    let root: HTMLElement;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [UiModule, TranslateModule.forRoot(), TablerIconStub],
            declarations: [AgentRunSummaryComponent]
        }).compileComponents();
        fixture = TestBed.createComponent(AgentRunSummaryComponent);
        root = fixture.nativeElement as HTMLElement;
    });

    function render(inputs: Record<string, unknown>): void {
        Object.entries(inputs).forEach(([name, value]) =>
            fixture.componentRef.setInput(name, value)
        );
        fixture.detectChanges();
    }

    function text(testId: string): string {
        return root.querySelector(`[data-testid="${testId}"]`)?.textContent?.trim() ?? '';
    }

    it('shows the run model and the totals of the run', () => {
        render({ model: 'openai/kimi', stats: STATS });

        expect(text('run-model')).toBe('openai/kimi');
        expect(text('run-tokens')).toBe('206,493');
        expect(text('run-duration')).toBe('1m 39s');
        expect(text('run-tool-calls')).toBe('26');
    });

    it('splits the tokens by model only when more than one model ran', () => {
        render({ stats: STATS });
        expect(root.querySelector('[data-testid="run-tokens-by-model"]')).toBeNull();

        render({
            stats: { ...STATS, tokensByModel: { 'openai/kimi': 30, 'anthropic/sonnet': 100 } }
        });

        const rows = Array.from(root.querySelectorAll('[data-testid="run-model-tokens"]')).map(
            row =>
                Array.from(row.querySelectorAll('span'))
                    .map(cell => cell.textContent?.trim())
                    .join(' ')
        );
        expect(rows).toEqual(['anthropic/sonnet 100', 'openai/kimi 30']);
        expect(root.querySelector('[data-testid="run-tokens-by-model"]')?.textContent).toContain(
            'AGENT.STATS.TOKENS_BY_MODEL'
        );
    });

    it('offers the short workflow switch while it can change', () => {
        render({ canChangeWorkflow: true });
        const toggled = vi.fn();
        fixture.componentInstance.workflowToggled.subscribe(toggled);

        (root.querySelector('[data-testid="run-short-workflow"]') as HTMLInputElement).click();

        expect(toggled).toHaveBeenCalled();
    });

    it('shows the workflow with a lock once it can no longer change', () => {
        render({ isShortWorkflow: true, isWorkflowLockShown: true });

        expect(root.querySelector('[data-testid="run-short-workflow"]')).toBeNull();
        expect(text('run-workflow')).toBe('AGENT.WORKFLOW.SHORT_VALUE');
        expect(root.querySelector('[data-testid="run-workflow-locked"]')).not.toBeNull();
    });
});

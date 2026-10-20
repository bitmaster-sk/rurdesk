import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { Subject, of } from 'rxjs';
import { describe, expect, it } from 'vitest';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { AgentRunApi } from '../../api/agent-run.api.service';
import { RunStats } from '../../model/agent-run.model';
import { RunStatsPanelComponent } from './run-stats-panel.component';

const STATS: RunStats = {
    totalTokensUsed: 130,
    totalDurationMs: 5000,
    totalToolCallsCount: 4,
    attemptsPerStage: { design: 1 },
    failedAttempts: 0,
    tokensByModel: { 'ollama/qwen3:8b': 100, 'openai/kimi': 30 }
};

async function render(stats: RunStats): Promise<HTMLElement> {
    await TestBed.configureTestingModule({
        imports: [TranslateModule.forRoot()],
        declarations: [RunStatsPanelComponent],
        providers: [
            { provide: AgentRunApi, useValue: { loadStats$: () => of(stats) } },
            { provide: NoticeService, useValue: { agentStats$: new Subject() } }
        ]
    }).compileComponents();
    const fixture = TestBed.createComponent(RunStatsPanelComponent);
    fixture.componentRef.setInput('idRun', 5);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
}

describe('RunStatsPanelComponent (browser)', () => {
    it('splits the tokens by the model that spent them', async () => {
        const root = await render(STATS);

        const rows = Array.from(root.querySelectorAll('[data-testid="stats-model-row"]')).map(row =>
            Array.from(row.querySelectorAll('span'))
                .map(cell => cell.textContent?.trim())
                .join(' ')
        );
        expect(rows).toEqual(['ollama/qwen3:8b 100', 'openai/kimi 30']);
    });

    it('leaves the model section out when no task recorded a model', async () => {
        const root = await render({ ...STATS, tokensByModel: {} });

        expect(root.querySelector('[data-testid="stats-models"]')).toBeNull();
    });
});

import { CommonModule } from '@angular/common';
import { Directive, Input } from '@angular/core';
import { ChartData } from 'chart.js';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { By } from '@angular/platform-browser';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { beforeEach, describe, expect, it } from 'vitest';
import { UiModule } from 'src/app/ui/ui.module';
import { SprintState } from '../../constants/sprint-state.enum';
import { SprintUnit } from '../../constants/sprint-unit.enum';
import { SprintBurndown, SprintBurndownDay } from '../../model/sprint-burndown.model';
import { Sprint } from '../../model/sprint.model';
import { SprintStats } from '../../model/sprint-stats.model';
import { SprintVelocity } from '../../model/sprint-velocity.model';
import { SprintChartsBandComponent } from './sprint-charts-band.component';

import en from 'src/assets/i18n/en.json';

const BURNDOWN = '[data-testid="sprint-burndown-chart"]';
const VELOCITY = '[data-testid="sprint-velocity-chart"]';

function makeSprint(): Sprint {
    return {
        idSprint: 4,
        idProject: 1,
        name: 'Sprint 4',
        startAt: '2026-05-01T00:00:00Z',
        endAt: '2026-05-05T00:00:00Z',
        state: SprintState.Planned
    };
}

function makeDay(date: string, scope: number, done: number, snapshot: boolean): SprintBurndownDay {
    return {
        day: `${date}T00:00:00Z`,
        totalPoints: scope,
        donePoints: done,
        remainingPoints: scope - done,
        totalIssues: scope * 10,
        doneIssues: done * 10,
        remainingIssues: (scope - done) * 10,
        snapshot
    };
}

function makeBurndown(days: SprintBurndownDay[]): SprintBurndown {
    return {
        idSprint: 4,
        startAt: '2026-05-01T00:00:00Z',
        endAt: '2026-05-05T00:00:00Z',
        state: SprintState.Planned,
        days
    };
}

function makeVelocity(
    name: string,
    done: number,
    planned?: number,
    doneIssues: number = done
): SprintVelocity {
    return {
        idSprint: name.length,
        name,
        endAt: '2026-04-01T00:00:00Z',
        donePoints: done,
        doneIssues,
        plannedPoints: planned,
        plannedIssues: planned,
        frozen: planned !== undefined
    };
}

@Directive({ selector: 'canvas[baseChart]', standalone: true })
class BaseChartStub {
    @Input() public type?: string;
    @Input() public data?: ChartData;
    @Input() public options?: unknown;
    @Input() public plugins?: unknown;
}

describe('SprintChartsBandComponent', () => {
    let fixture: ComponentFixture<SprintChartsBandComponent>;

    function render(inputs: {
        sprint?: Sprint | null;
        burndown?: SprintBurndown | null;
        velocities?: SprintVelocity[];
        projectName?: string;
        isLoading?: boolean;
        stats?: SprintStats | null;
    }): void {
        fixture.componentRef.setInput('sprint', inputs.sprint ?? null);
        fixture.componentRef.setInput('stats', inputs.stats ?? null);
        fixture.componentRef.setInput('burndown', inputs.burndown ?? null);
        fixture.componentRef.setInput('velocities', inputs.velocities ?? []);
        fixture.componentRef.setInput('projectName', inputs.projectName ?? 'Apollo');
        fixture.componentRef.setInput('isLoading', inputs.isLoading ?? false);
        fixture.componentRef.setInput('unit', SprintUnit.Points);
        fixture.detectChanges();
    }

    function text(selector: string): string {
        const el = fixture.debugElement.query(By.css(selector));
        return el ? el.nativeElement.textContent.trim().replace(/\s+/g, ' ') : '';
    }

    function chartData(selector: string): ChartData {
        const canvas = fixture.debugElement.query(By.css(selector));
        expect(canvas, `${selector} is rendered`).toBeTruthy();
        return canvas.injector.get(BaseChartStub).data!;
    }

    function clickOption(label: string): void {
        const option = fixture.debugElement
            .queryAll(By.css('[data-testid="sprint-charts-mode"] button'))
            .find(candidate => candidate.nativeElement.textContent.trim() === label);
        expect(option, `the ${label} option is rendered`).toBeTruthy();
        option!.nativeElement.click();
        fixture.detectChanges();
    }

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [SprintChartsBandComponent],
            imports: [CommonModule, FormsModule, TranslateModule.forRoot(), UiModule, BaseChartStub]
        }).compileComponents();

        const translate = TestBed.inject(TranslateService);
        translate.setTranslation('en', en);
        translate.use('en');

        fixture = TestBed.createComponent(SprintChartsBandComponent);
    });

    it('names the cycle and its window in the header', () => {
        render({
            sprint: makeSprint(),
            burndown: makeBurndown([makeDay('2026-05-01', 20, 0, true)])
        });

        expect(text('[data-testid="sprint-charts-header"]')).toContain('Sprint 4');
        expect(text('[data-testid="sprint-charts-header"]')).toContain('May 1');
    });

    it('names the project on the backlog tab and renders velocity only', () => {
        render({ sprint: null, velocities: [makeVelocity('S1', 8, 10)] });

        expect(text('[data-testid="sprint-charts-header"]')).toContain('Apollo');
        expect(fixture.debugElement.query(By.css(BURNDOWN))).toBeNull();
        expect(fixture.debugElement.query(By.css(VELOCITY))).not.toBeNull();
    });

    it('reports the largest scope increase with its day', () => {
        render({
            sprint: makeSprint(),
            burndown: makeBurndown([
                makeDay('2026-05-01', 20, 0, true),
                makeDay('2026-05-02', 21, 2, true),
                makeDay('2026-05-03', 24, 6, true)
            ])
        });

        expect(text('[data-testid="sprint-charts-scope"]')).toBe('+4 pts added');
        expect(fixture.nativeElement.textContent).toContain('day 3 of 4');
    });

    it('keeps the slot with a dash when scope never moved', () => {
        render({
            sprint: makeSprint(),
            burndown: makeBurndown([
                makeDay('2026-05-01', 20, 0, true),
                makeDay('2026-05-02', 20, 4, true)
            ])
        });

        expect(text('[data-testid="sprint-charts-scope"]')).toBe('unchanged');
    });

    it('renders the empty-history overlay instead of a canvas', () => {
        render({ sprint: makeSprint(), burndown: makeBurndown([]) });

        expect(fixture.debugElement.query(By.css(BURNDOWN))).toBeNull();
        expect(text('[data-testid="sprint-burndown-empty"]')).toContain('No history');
    });

    it('shows no empty overlay while the payload is still loading', () => {
        render({ sprint: makeSprint(), burndown: null, isLoading: true });

        expect(
            fixture.debugElement.query(By.css('[data-testid="sprint-burndown-empty"]'))
        ).toBeNull();
    });

    it('swaps the rendered series when burnup is selected', () => {
        render({
            sprint: makeSprint(),
            burndown: makeBurndown([
                makeDay('2026-05-01', 20, 0, true),
                makeDay('2026-05-02', 20, 6, true)
            ])
        });

        expect(chartData(BURNDOWN).datasets[0].label).toBe('Remaining');
        expect(chartData(BURNDOWN).datasets[0].data).toEqual([20, 14]);

        clickOption('Burnup');

        expect(chartData(BURNDOWN).datasets[0].label).toBe('Done');
        expect(chartData(BURNDOWN).datasets[0].data).toEqual([0, 6]);
    });

    it('draws no planned bar for a cycle closed before snapshots existed', () => {
        render({
            sprint: makeSprint(),
            velocities: [makeVelocity('S1', 8), makeVelocity('S2', 11, 13)]
        });
        expect(chartData(VELOCITY).datasets[1].data).toEqual([null, 13]);
        expect(text('[data-testid="sprint-velocity-fallback"]')).toContain('live numbers');
    });

    it('keeps the trend line off while the older window is a single cycle', () => {
        render({
            sprint: makeSprint(),
            velocities: [1, 2, 3].map(n => makeVelocity(`S${n}`, n * 2, n * 2))
        });

        expect(text('[data-testid="sprint-charts-avg"]')).toBe('5 pts avg');
        expect(fixture.nativeElement.textContent).not.toContain('up from');
    });

    it('names the real size of the older window', () => {
        render({
            sprint: makeSprint(),
            velocities: [1, 2, 3, 4, 5, 6, 7].map(n => makeVelocity(`S${n}`, n * 2, n * 2))
        });

        expect(fixture.nativeElement.textContent).toContain('up from 4 in the 3 before');
    });

    it('follows the forced Tasks unit when the cycle has no points', () => {
        render({
            sprint: makeSprint(),
            stats: {
                totalPoints: 0,
                donePoints: 0,
                startPoints: 0,
                progressPoints: 0,
                totalIssues: 4,
                doneIssues: 1,
                startIssues: 3,
                progressIssues: 0,
                pointedIssues: 0
            },
            burndown: makeBurndown([makeDay('2026-05-01', 20, 5, true)]),
            velocities: [makeVelocity('S1', 0, 0)]
        });

        expect(chartData(BURNDOWN).datasets[0].data).toEqual([150]);
        expect(text('[data-testid="sprint-charts-scope"]')).toContain('unchanged');
    });

    it('forces tasks on the backlog tab of a project that has no points', () => {
        render({
            sprint: null,
            stats: {
                totalPoints: 0,
                donePoints: 0,
                startPoints: 0,
                progressPoints: 0,
                totalIssues: 42,
                doneIssues: 0,
                startIssues: 42,
                progressIssues: 0,
                pointedIssues: 0
            },
            velocities: [makeVelocity('S1', 0, undefined, 4), makeVelocity('S2', 0, undefined, 6)]
        });

        expect(chartData(VELOCITY).datasets[0].data).toEqual([4, 6]);
    });

    it('says the velocity fell when it fell', () => {
        render({
            sprint: makeSprint(),
            velocities: [20, 20, 20, 20, 20, 8, 8, 8, 8, 8].map((n, index) =>
                makeVelocity(`S${index}`, n, n)
            )
        });

        expect(text('[data-testid="sprint-charts-avg"]')).toBe('8 pts avg');
        expect(fixture.nativeElement.textContent).toContain('down from 20 in the 5 before');
    });

    it('spells out how late the forecast finish is', () => {
        render({
            sprint: makeSprint(),
            stats: {
                totalPoints: 8,
                donePoints: 3,
                startPoints: 5,
                progressPoints: 0,
                totalIssues: 2,
                doneIssues: 1,
                startIssues: 1,
                progressIssues: 0,
                pointedIssues: 2
            },
            burndown: makeBurndown([makeDay('2026-05-01', 8, 3, true)])
        });

        expect(text('[data-testid="sprint-charts-forecast"]')).toContain('finish');
        expect(fixture.nativeElement.textContent).toContain('days past the sprint end');
    });
});

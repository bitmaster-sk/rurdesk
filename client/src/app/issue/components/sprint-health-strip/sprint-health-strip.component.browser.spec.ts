import { CommonModule } from '@angular/common';
import { SprintState } from '../../constants/sprint-state.enum';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { By } from '@angular/platform-browser';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { beforeEach, describe, expect, it } from 'vitest';
import { SprintUnit } from '../../constants/sprint-unit.enum';
import { Sprint } from '../../model/sprint.model';
import { SprintVelocity } from '../../model/sprint-velocity.model';
import { SprintStats } from '../../model/sprint-stats.model';
import { UiModule } from 'src/app/ui/ui.module';
import { UiTooltipDirective } from 'src/app/ui/directives/tooltip.directive';
import { TablerIconStub } from 'src/testing/stubs';
import { SprintHealthStripComponent } from './sprint-health-strip.component';

import en from 'src/assets/i18n/en.json';

function makeStats(partial: Partial<SprintStats>): SprintStats {
    return {
        totalPoints: 0,
        donePoints: 0,
        startPoints: 0,
        progressPoints: 0,
        totalIssues: 0,
        doneIssues: 0,
        startIssues: 0,
        progressIssues: 0,
        pointedIssues: 1,
        ...partial
    };
}

function makeSprint(
    startAt: string,
    endAt: string,
    state: SprintState = SprintState.Planned
): Sprint {
    return { idSprint: 4, idProject: 1, name: 'Sprint 4', startAt, endAt, state };
}

function makeVelocity(donePoints: number, doneIssues: number): SprintVelocity {
    return { idSprint: 1, name: 'v', endAt: '', donePoints, doneIssues, frozen: false };
}

function daysFromToday(offset: number): string {
    const now = new Date();
    const day = Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate());
    return new Date(day + offset * 86_400_000).toISOString();
}

describe('SprintHealthStripComponent', () => {
    let fixture: ComponentFixture<SprintHealthStripComponent>;

    function render(
        sprint: Sprint | null,
        stats: SprintStats | null,
        velocities: SprintVelocity[] = [],
        unit: SprintUnit = SprintUnit.Points
    ): void {
        fixture.componentRef.setInput('sprint', sprint);
        fixture.componentRef.setInput('stats', stats);
        fixture.componentRef.setInput('velocities', velocities);
        fixture.componentRef.setInput('unit', unit);
        fixture.detectChanges();
    }

    function text(selector: string): string {
        const el = fixture.debugElement.query(By.css(selector));
        return el ? el.nativeElement.textContent.trim().replace(/\s+/g, ' ') : '';
    }

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [SprintHealthStripComponent],
            imports: [
                CommonModule,
                FormsModule,
                TranslateModule.forRoot(),
                UiModule,
                TablerIconStub
            ]
        }).compileComponents();

        const translate = TestBed.inject(TranslateService);
        translate.setTranslation('en', en);
        translate.use('en');

        fixture = TestBed.createComponent(SprintHealthStripComponent);
    });

    it('renders a planned sprint as a commitment', () => {
        render(
            makeSprint(daysFromToday(3), daysFromToday(17)),
            makeStats({ totalPoints: 21, startPoints: 21, pointedIssues: 5 })
        );
        expect(text('[data-testid="sprint-health-progress"]')).toBe('21 pts planned');
        expect(fixture.nativeElement.textContent).toContain('starts in 3 days');
    });

    it('shows the over-committed chip against the average', () => {
        render(
            makeSprint(daysFromToday(3), daysFromToday(17)),
            makeStats({ totalPoints: 21, startPoints: 21, pointedIssues: 5 }),
            [makeVelocity(10, 10)]
        );
        expect(text('[data-testid="sprint-health-verdict"]')).toContain('over-committed vs avg 10');
    });

    it('rounds an average that does not divide evenly', () => {
        render(
            makeSprint(daysFromToday(3), daysFromToday(17)),
            makeStats({ totalPoints: 21, startPoints: 21, pointedIssues: 5 }),
            [
                makeVelocity(5, 5),
                makeVelocity(5, 5),
                makeVelocity(1, 1),
                makeVelocity(2, 2),
                makeVelocity(2, 2)
            ]
        );
        expect(text('[data-testid="sprint-health-verdict"]')).toContain(
            'over-committed vs avg 1.7'
        );
    });

    it('builds the average from cycles that scored, not from ones that never used the unit', () => {
        // Points were adopted three cycles ago; the older cycles scored zero points
        // and must not drag a routine 21-point commitment into "over-committed".
        render(
            makeSprint(daysFromToday(3), daysFromToday(17)),
            makeStats({ totalPoints: 21, startPoints: 21, pointedIssues: 5 }),
            [
                makeVelocity(0, 4),
                makeVelocity(0, 5),
                makeVelocity(0, 6),
                makeVelocity(20, 7),
                makeVelocity(22, 8)
            ]
        );
        expect(text('[data-testid="sprint-health-verdict"]')).toBe('');
    });

    it('says day, not days, when a single day is left', () => {
        render(
            makeSprint(daysFromToday(-12), daysFromToday(2)),
            makeStats({ totalPoints: 21, donePoints: 16, progressPoints: 5, pointedIssues: 5 })
        );
        expect(fixture.nativeElement.textContent).toContain('1 day left');
        expect(fixture.nativeElement.textContent).not.toContain('1 days left');
    });

    it('says day, not days, when a cycle starts tomorrow', () => {
        render(
            makeSprint(daysFromToday(1), daysFromToday(15)),
            makeStats({ totalIssues: 1, startIssues: 1, pointedIssues: 0 })
        );
        expect(fixture.nativeElement.textContent).toContain('starts in 1 day');
        expect(fixture.nativeElement.textContent).not.toContain('starts in 1 days');
    });

    it('makes the unit agree with the number it follows', () => {
        render(
            makeSprint(daysFromToday(3), daysFromToday(17)),
            makeStats({ totalPoints: 1, startPoints: 1, pointedIssues: 1 })
        );
        expect(text('[data-testid="sprint-health-progress"]')).toBe('1 pt planned');
    });

    it('leaves finished work out of the backlog count', () => {
        render(
            null,
            makeStats({
                totalIssues: 12,
                doneIssues: 10,
                totalPoints: 30,
                donePoints: 25,
                pointedIssues: 12
            })
        );
        expect(text('[data-testid="sprint-health-progress"]')).toBe(
            '2 tasks · 5 pts open, in no sprint'
        );
    });

    it('hides the unit toggle on the backlog, where it would do nothing', () => {
        render(null, makeStats({ totalIssues: 3, pointedIssues: 3 }));
        expect(fixture.debugElement.query(By.css('[data-testid="sprint-health-unit"]'))).toBeNull();
    });

    it('shows a closed cycle as fully done, even after a task is reopened', () => {
        render(
            makeSprint(daysFromToday(-20), daysFromToday(-6), SprintState.Closed),
            makeStats({
                totalPoints: 21,
                donePoints: 13,
                startPoints: 8,
                pointedIssues: 5
            })
        );
        expect(text('[data-testid="sprint-health-progress"]')).toBe('13 pts done');
        const done = fixture.debugElement.query(By.css('.segment--done'))
            .nativeElement as HTMLElement;
        expect(done.style.width).toBe('100%');
    });

    it('counts a single backlog task in the singular', () => {
        render(null, makeStats({ totalIssues: 1, totalPoints: 1, pointedIssues: 1 }));
        expect(text('[data-testid="sprint-health-progress"]')).toBe(
            '1 task · 1 pt open, in no sprint'
        );
    });

    it('shows nothing but the name and window until the stats arrive', () => {
        render(makeSprint(daysFromToday(-9), daysFromToday(5)), null);
        expect(text('[data-testid="sprint-health-progress"]')).toBe('');
        expect(fixture.nativeElement.textContent).not.toContain('no points set');
        expect(fixture.nativeElement.textContent).toContain('Sprint 4');
    });

    it('disables the unit toggle when the unit is forced', () => {
        render(
            makeSprint(daysFromToday(-9), daysFromToday(5)),
            makeStats({ totalIssues: 20, doneIssues: 9, startIssues: 11, pointedIssues: 0 })
        );
        const toggle = fixture.debugElement.query(By.css('[data-testid="sprint-health-unit"]'));
        expect(toggle.componentInstance.disabled()).toBe(true);
        expect(toggle.injector.get(UiTooltipDirective).uiTooltip()).toBe(
            'no points set — showing tasks'
        );
    });

    it('leaves the unit toggle usable when the cycle has points', () => {
        render(
            makeSprint(daysFromToday(-9), daysFromToday(5)),
            makeStats({ totalPoints: 21, donePoints: 5, pointedIssues: 5 })
        );
        const toggle = fixture.debugElement.query(By.css('[data-testid="sprint-health-unit"]'));
        expect(toggle.componentInstance.disabled()).toBe(false);
        expect(toggle.injector.get(UiTooltipDirective).uiTooltip()).toBe('');
    });

    it('re-renders the interpolated unit when the language changes', () => {
        const translate = TestBed.inject(TranslateService);
        translate.setTranslation('sk', {
            ISSUE: {
                KANBAN: {
                    SPRINTS: {
                        HEALTH_PLANNED: '{{count}} {{unit}} naplánovaných',
                        UNIT_POINTS_SHORT: { SINGULAR: 'bod', PLURAL: 'bodov' }
                    }
                }
            }
        });
        render(
            makeSprint(daysFromToday(3), daysFromToday(17)),
            makeStats({ totalPoints: 21, startPoints: 21, pointedIssues: 5 })
        );
        expect(text('[data-testid="sprint-health-progress"]')).toBe('21 pts planned');

        translate.use('sk');
        fixture.detectChanges();

        expect(text('[data-testid="sprint-health-progress"]')).toBe('21 bodov naplánovaných');
    });

    it('says it is too early to forecast on day 2', () => {
        render(
            makeSprint(daysFromToday(-1), daysFromToday(13)),
            makeStats({ totalPoints: 21, donePoints: 1, progressPoints: 20, pointedIssues: 5 })
        );
        expect(text('[data-testid="sprint-health-verdict"]')).toBe('too early to forecast');
    });

    it('renders an on-track running sprint', () => {
        render(
            makeSprint(daysFromToday(-9), daysFromToday(5)),
            makeStats({
                totalPoints: 21,
                donePoints: 16,
                progressPoints: 3,
                startPoints: 2,
                pointedIssues: 5
            })
        );
        expect(text('[data-testid="sprint-health-progress"]')).toBe('16/21 pts');
        expect(text('[data-testid="sprint-health-verdict"]')).toContain('on track');
        expect(fixture.nativeElement.textContent).toContain('4 days left');
    });

    it('renders a behind running sprint with the gap in the effective unit', () => {
        render(
            makeSprint(daysFromToday(-9), daysFromToday(5)),
            makeStats({
                totalPoints: 21,
                donePoints: 13,
                progressPoints: 4,
                startPoints: 4,
                pointedIssues: 5
            })
        );
        expect(text('[data-testid="sprint-health-verdict"]')).toContain('behind 3 pts');
        expect(fixture.nativeElement.textContent).toContain('pace 1.3/day, needs 2');
    });

    it('renders a closed sprint with no denominator and no not-started segment', () => {
        render(
            makeSprint(daysFromToday(-20), daysFromToday(-6), SprintState.Closed),
            makeStats({
                totalPoints: 14,
                donePoints: 14,
                totalIssues: 6,
                doneIssues: 6,
                pointedIssues: 6
            })
        );
        expect(text('[data-testid="sprint-health-progress"]')).toBe('14 pts done');
        expect(fixture.nativeElement.textContent).toContain('closed');
        const todo = fixture.debugElement.query(By.css('.segment--todo'));
        expect(todo.nativeElement.style.width).toBe('0%');
    });

    it('renders the backlog summary in both units', () => {
        render(null, makeStats({ totalPoints: 96, totalIssues: 42, pointedIssues: 20 }));
        expect(text('[data-testid="sprint-health-progress"]')).toBe(
            '42 tasks · 96 pts open, in no sprint'
        );
    });

    it('sums the segment widths to 100%', () => {
        render(
            makeSprint(daysFromToday(-9), daysFromToday(5)),
            makeStats({
                totalPoints: 20,
                donePoints: 5,
                progressPoints: 7,
                startPoints: 8,
                pointedIssues: 5
            })
        );
        const widths = fixture.debugElement
            .queryAll(By.css('.bar__segment'))
            .map(el => parseFloat(el.nativeElement.style.width));
        expect(widths.reduce((a, b) => a + b, 0)).toBeCloseTo(100, 5);
    });

    it('renders no NaN when nothing is committed', () => {
        render(makeSprint(daysFromToday(-9), daysFromToday(5)), makeStats({ pointedIssues: 1 }));
        expect(fixture.nativeElement.textContent).not.toContain('NaN');
        const widths = fixture.debugElement
            .queryAll(By.css('.bar__segment'))
            .map(el => el.nativeElement.style.width);
        expect(widths.every((w: string) => w === '0%')).toBe(true);
    });

    it('forces tasks and shows the hint when nothing is pointed', () => {
        render(
            makeSprint(daysFromToday(-9), daysFromToday(5)),
            makeStats({ totalIssues: 20, doneIssues: 9, startIssues: 11, pointedIssues: 0 }),
            [],
            SprintUnit.Points
        );
        expect(text('[data-testid="sprint-health-progress"]')).toBe('9/20 tasks');
        expect(fixture.nativeElement.textContent).toContain('no points set');
    });

    it('emits the picked unit through the model', () => {
        render(
            makeSprint(daysFromToday(-9), daysFromToday(5)),
            makeStats({ totalPoints: 20, donePoints: 5, pointedIssues: 5 })
        );
        const choice = fixture.debugElement.query(By.css('ui-choice'));
        choice.triggerEventHandler('ngModelChange', SprintUnit.Issues);
        fixture.detectChanges();
        expect(fixture.componentInstance.unit()).toBe(SprintUnit.Issues);
    });

    it('restores the fraction and the rolled-over chip for a frozen closed cycle', () => {
        render(
            makeSprint(daysFromToday(-20), daysFromToday(-6), SprintState.Closed),
            makeStats({
                totalPoints: 14,
                donePoints: 14,
                totalIssues: 14,
                doneIssues: 14,
                pointedIssues: 14,
                rolledOverIssues: 6,
                frozenTotalPoints: 20,
                frozenDonePoints: 14,
                frozenTotalIssues: 20,
                frozenDoneIssues: 14,
                frozenPointedIssues: 20
            })
        );
        expect(text('[data-testid="sprint-health-progress"]')).toBe('14/20 pts');
        expect(text('[data-testid="sprint-health-verdict"]')).toContain('6 tasks rolled over');
        const done = fixture.debugElement.query(By.css('.segment--done'))
            .nativeElement as HTMLElement;
        expect(done.style.width).toBe('70%');
    });

    it('has no rolled-over chip for a closed cycle without frozen numbers', () => {
        render(
            makeSprint(daysFromToday(-20), daysFromToday(-6), SprintState.Closed),
            makeStats({ totalPoints: 14, donePoints: 14, pointedIssues: 6 })
        );
        expect(
            fixture.debugElement.query(By.css('[data-testid="sprint-health-verdict"]'))
        ).toBeNull();
    });
});

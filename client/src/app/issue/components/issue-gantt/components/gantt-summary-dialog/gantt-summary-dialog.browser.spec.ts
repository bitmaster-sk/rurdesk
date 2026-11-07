import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { CommonModule } from '@angular/common';
import { TranslateModule } from '@ngx-translate/core';
import { UiDialogComponent } from 'src/app/ui/components/dialog/dialog.component';
import { UiButtonComponent } from 'src/app/ui/components/button/button.component';
import { SecDurationPipe } from 'src/app/shared/duration/sec-duration.pipe';
import { TablerIconStub } from 'src/testing/stubs';
import { GanttSummaryDialogComponent } from './gantt-summary-dialog';
import { GanttSummaryChoice } from '../../constants/gantt-summary-choice.enum';

describe('GanttSummaryDialogComponent', () => {
    let fixture: ComponentFixture<GanttSummaryDialogComponent>;
    const chosen: GanttSummaryChoice[] = [];
    let cancelled = 0;
    const button = (id: string) =>
        document.body.querySelector<HTMLElement>(`[data-testid="${id}"] button`)!;

    beforeEach(() => {
        chosen.length = 0;
        cancelled = 0;
        TestBed.configureTestingModule({
            declarations: [
                GanttSummaryDialogComponent,
                UiDialogComponent,
                UiButtonComponent,
                SecDurationPipe
            ],
            imports: [CommonModule, TranslateModule.forRoot(), TablerIconStub]
        });
        fixture = TestBed.createComponent(GanttSummaryDialogComponent);
        fixture.componentRef.setInput('title', 'Vývoj');
        fixture.componentRef.setInput('estimated', 7200);
        fixture.componentInstance.chosen.subscribe(c => chosen.push(c));
        fixture.componentInstance.cancelled.subscribe(() => cancelled++);
        fixture.detectChanges();
    });

    afterEach(() => fixture.destroy());

    it('asks what to do with the own estimate', () => {
        expect(
            document.body.querySelector('[data-testid="gantt-summary-dialog-text"]')?.textContent
        ).toContain('ISSUE.GANTT.MAKE_SUMMARY.TEXT');
        expect(button('gantt-summary-dialog-move').textContent).toContain(
            'ISSUE.GANTT.MAKE_SUMMARY.MOVE'
        );
    });

    it('reports the chosen option', () => {
        button('gantt-summary-dialog-move').click();
        button('gantt-summary-dialog-discard').click();
        expect(chosen).toEqual([GanttSummaryChoice.MoveToSubtask, GanttSummaryChoice.Discard]);
        expect(cancelled).toBe(0);
    });

    it('cancels from the cancel button', () => {
        button('gantt-summary-dialog-cancel').click();
        expect(cancelled).toBe(1);
        expect(chosen).toEqual([]);
    });
});

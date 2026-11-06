import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { Component, input } from '@angular/core';
import { DragDropModule } from '@angular/cdk/drag-drop';
import { FormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { UiChoiceComponent } from 'src/app/ui/components/choice/choice.component';
import { SecDurationPipe } from 'src/app/shared/duration/sec-duration.pipe';
import { GanttWbsPanelComponent } from './gantt-wbs-panel';
import { GanttWbsDrop } from '../../entity/gantt-wbs-drop.entity';
import { GanttRowDragMove } from './entity/gantt-wbs-panel.entity';
import { GanttTimelineService } from '../../service/gantt-timeline.service';
import { GanttTreeConverter } from '../../converter/gantt-tree.converter';
import { ScheduledIssue } from '../../../../model/extended-issue.model';
import { Fixtures } from 'src/testing/fixtures';
import { AvatarStub, TablerIconStub, UiButtonStub } from 'src/testing/stubs';

@Component({ selector: 'app-state-badge', template: '', standalone: true })
class StateBadgeStub {
    public readonly state = input<unknown>();
    public readonly size = input<string>();
}

const task = (idIssuePublic: number, title: string): ScheduledIssue => ({
    ...Fixtures.extendedIssue({ idIssuePublic, title }),
    scheduledAt: new Date('2025-01-15T00:00:00Z')
});

describe('GanttWbsPanelComponent tree rows', () => {
    let fixture: ComponentFixture<GanttWbsPanelComponent>;
    let el: HTMLElement;

    beforeEach(() => {
        TestBed.configureTestingModule({
            declarations: [GanttWbsPanelComponent, UiChoiceComponent, SecDurationPipe],
            imports: [
                TranslateModule.forRoot(),
                FormsModule,
                DragDropModule,
                StateBadgeStub,
                AvatarStub,
                TablerIconStub,
                UiButtonStub
            ],
            providers: [{ provide: GanttTimelineService, useValue: { rowHeight: () => 40 } }]
        });
        fixture = TestBed.createComponent(GanttWbsPanelComponent);
        const parents = new Map([[2, 1]]);
        const rows = GanttTreeConverter.visibleRows(
            [task(1, 'Vývoj'), task(2, 'Backend'), task(3, 'Test')],
            parents,
            {
                collapsed: new Set(),
                outlineLevel: null
            }
        );
        fixture.componentRef.setInput('rows', rows);
        fixture.componentRef.setInput('parents', parents);
        fixture.componentRef.setInput('backlogTasks', []);
        fixture.detectChanges();
        el = fixture.nativeElement as HTMLElement;
    });

    it('indents children and numbers every row', () => {
        const rows = [...el.querySelectorAll<HTMLElement>('.wbs-row')];
        expect(rows.map(r => r.querySelector('.wbs-row--code')?.textContent?.trim())).toEqual([
            '1',
            '1.1',
            '2'
        ]);
        expect(rows[1].style.paddingLeft).toBe('24px');
    });

    it('shows a chevron only on rows with children', () => {
        const chevrons = el.querySelectorAll('.wbs-row--chevron');
        expect(chevrons.length).toBe(1);
        expect(chevrons[0].getAttribute('aria-expanded')).toBe('true');
    });

    it('toggles collapse from the chevron without opening the task', () => {
        const toggled: number[] = [];
        const clicked: unknown[] = [];
        fixture.componentInstance.collapseToggled.subscribe(id => toggled.push(id));
        fixture.componentInstance.taskClicked.subscribe(e => clicked.push(e));
        el.querySelector<HTMLButtonElement>('.wbs-row--chevron')!.click();
        expect(toggled).toEqual([1]);
        expect(clicked).toEqual([]);
    });

    it('indents a row dragged sideways over its own placeholder', () => {
        const own = el.querySelectorAll<HTMLElement>('.wbs-row')[2];
        own.style.pointerEvents = 'none';
        const rect = own.getBoundingClientRect();
        const move: GanttRowDragMove = {
            pointerPosition: { x: rect.left + 60, y: rect.top + rect.height / 2 },
            distance: { x: 40, y: 0 },
            source: { getPlaceholderElement: () => own }
        };
        const dropped: GanttWbsDrop[] = [];
        fixture.componentInstance.treeDropped.subscribe(d => dropped.push(d));

        fixture.componentInstance.onRowDragStarted(3);
        fixture.componentInstance.onRowDragMoved(move);
        fixture.componentInstance.onScheduledDrop();

        expect(dropped).toEqual([{ movedId: 3, targetId: 3, kind: 'indent' }]);
    });

    it('shows the drop hint below the dragged row and hides it after the drop', () => {
        const own = el.querySelectorAll<HTMLElement>('.wbs-row')[2];
        own.style.pointerEvents = 'none';
        const rect = own.getBoundingClientRect();
        const pointer = { x: rect.left + 60, y: rect.top + rect.height / 2 };
        fixture.componentInstance.onRowDragStarted(3);
        fixture.componentInstance.onRowDragMoved({
            pointerPosition: pointer,
            distance: { x: 40, y: 0 },
            source: { getPlaceholderElement: () => own }
        });
        fixture.detectChanges();

        const tip = el.querySelector<HTMLElement>('.wbs-panel--drop-tip')!;
        expect(tip.textContent?.trim()).toBe('ISSUE.GANTT.INDENT');
        expect(tip.style.left).toBe(`${pointer.x}px`);
        expect(tip.style.top).toBe(`${rect.bottom + 4}px`);
        expect(own.querySelector('.wbs-panel--drop-tip')).toBeNull();

        fixture.componentInstance.onScheduledDrop();
        fixture.detectChanges();
        expect(el.querySelector('.wbs-panel--drop-tip')).toBeNull();
    });

    it('warns in the drop hint when the target would become a summary', () => {
        fixture.componentRef.setInput(
            'rows',
            GanttTreeConverter.visibleRows(
                [task(1, 'Vývoj'), task(2, 'Backend'), { ...task(3, 'Test'), estimated: 7200 }],
                new Map([[2, 1]]),
                { collapsed: new Set(), outlineLevel: null }
            )
        );
        fixture.componentInstance.dropTarget.set({
            movedId: 1,
            targetId: 3,
            kind: 'into',
            isAllowed: true
        });
        fixture.componentInstance.dropTipAt.set({ x: 10, y: 10 });
        fixture.detectChanges();

        expect(fixture.componentInstance.newSummary()?.idIssuePublic).toBe(3);
        expect(el.querySelector('.wbs-panel--drop-tip-note')?.textContent?.trim()).toBe(
            'ISSUE.GANTT.DROP_MAKES_SUMMARY'
        );

        fixture.componentInstance.dropTarget.set({
            movedId: 3,
            targetId: 2,
            kind: 'before',
            isAllowed: true
        });
        fixture.detectChanges();
        expect(el.querySelector('.wbs-panel--drop-tip-note')).toBeNull();
    });

    it('puts the outline levels in the panel header and resets them after a manual fold', async () => {
        fixture.componentRef.setInput('hasHierarchy', true);
        fixture.componentRef.setInput('outlineLevel', null);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();
        const levels: number[] = [];
        fixture.componentInstance.outlineLevelChange.subscribe(level => levels.push(level));
        const options = () => [
            ...el.querySelectorAll<HTMLButtonElement>('.wbs-panel--outline button')
        ];
        const pressed = () => options().map(o => o.getAttribute('aria-pressed'));

        expect(options().map(o => o.textContent?.trim())).toEqual([
            '1',
            '2',
            '3',
            'ISSUE.GANTT.OUTLINE_ALL'
        ]);
        expect(pressed()).not.toContain('true');

        options()[0].click();
        fixture.componentRef.setInput('outlineLevel', 1);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();
        expect(levels).toEqual([1]);
        expect(pressed()).toEqual(['true', 'false', 'false', 'false']);

        fixture.componentRef.setInput('outlineLevel', null);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();
        expect(pressed()).not.toContain('true');
    });

    it('shows the number of hidden subtasks next to a folded row', () => {
        const parents = new Map([[2, 1]]);
        fixture.componentRef.setInput(
            'rows',
            GanttTreeConverter.visibleRows([task(1, 'Vývoj'), task(2, 'Backend')], parents, {
                collapsed: new Set([1]),
                outlineLevel: null
            })
        );
        fixture.detectChanges();
        expect(el.querySelector('.wbs-row--count--hidden')?.textContent?.trim()).toBe('1');
    });
});

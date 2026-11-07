import { describe, it, expect } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { Component, input, output } from '@angular/core';
import { provideRouter } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { GanttTaskBarComponent } from './gantt-task-bar';
import { GanttTimelineService } from '../../service/gantt-timeline.service';
import { ScheduledIssue } from '../../../../model/extended-issue.model';
import { Fixtures } from 'src/testing/fixtures';
import { AvatarStub } from 'src/testing/stubs';

@Component({ selector: 'app-state-badge', template: '', standalone: true })
class StateBadgeStub {
    public readonly state = input<unknown>();
    public readonly size = input<string>();
}

@Component({ selector: 'app-gantt-connection-handle', template: '', standalone: true })
class ConnectionHandleStub {
    public readonly side = input<unknown>();
    public readonly taskId = input<number>();
    public readonly isOffset = input<boolean>(false);
    public readonly isDropActive = input<boolean>(false);
    public readonly dragStarted = output<unknown>();
}

const timelineServiceMock = {
    rowHeight: () => 64,
    toPixel: (date: Date) => date.getTime() / 3_600_000,
    toDate: (px: number) => new Date(px * 3_600_000),
    snapToNearest: (date: Date) => date
};

function render(over: Partial<ScheduledIssue>): HTMLElement {
    TestBed.configureTestingModule({
        declarations: [GanttTaskBarComponent],
        imports: [TranslateModule.forRoot(), StateBadgeStub, ConnectionHandleStub, AvatarStub],
        providers: [
            { provide: GanttTimelineService, useValue: timelineServiceMock },
            provideRouter([])
        ]
    });
    const fixture = TestBed.createComponent(GanttTaskBarComponent);
    const issue: ScheduledIssue = {
        ...Fixtures.extendedIssue({ idIssuePublic: 1, title: 'Vývoj', estimated: 7200 }),
        scheduledAt: new Date('2025-01-15T00:00:00Z'),
        ...over
    };
    fixture.componentRef.setInput('task', issue);
    fixture.componentRef.setInput('cardMode', 'GanttComfort');
    fixture.componentRef.setInput('rowIndex', 0);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
}

describe('GanttTaskBarComponent summary', () => {
    it('draws a summary as a thin bar without a resize handle', () => {
        const el = render({ isSummary: true, rollupEstimated: 3600, rollupTracked: 5400 });
        expect(el.querySelector('.gantt-bar--summary')).not.toBeNull();
        expect(el.querySelector('.gantt-bar--resize-handle')).toBeNull();
        expect(el.textContent).toContain('100 %');
        expect(el.textContent).toContain('+50 %');
    });

    it('does not clip the connection handles that sit outside a summary bar', () => {
        const el = render({ isSummary: true, rollupEstimated: 3600, rollupTracked: 1800 });
        const bar = el.querySelector<HTMLElement>('.gantt-bar--summary')!;
        expect(el.querySelectorAll('app-gantt-connection-handle').length).toBe(2);
        expect(getComputedStyle(bar).overflow).toBe('visible');
    });

    it('keeps the resize handle on a normal task', () => {
        const el = render({});
        expect(el.querySelector('.gantt-bar--summary')).toBeNull();
        expect(el.querySelector('.gantt-bar--resize-handle')).not.toBeNull();
    });
});

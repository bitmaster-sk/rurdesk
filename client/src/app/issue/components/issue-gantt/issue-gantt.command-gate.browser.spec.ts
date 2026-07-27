import { describe, it, expect, vi, beforeEach } from 'vitest';
import { createGanttFixture } from './gantt-testbed.helper';

describe('IssueGanttComponent keydown gate (TestBed)', () => {
    let fixture: any;
    let comp: any;
    let mocks: any;

    beforeEach(async () => {
        localStorage.clear();
        const result = await createGanttFixture();
        fixture = result.fixture;
        comp = result.comp;
        mocks = result.mocks;
    });

    function keyDown(key: string) {
        fixture.nativeElement.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true }));
        fixture.detectChanges();
    }

    it('ignores ArrowDown while a palette overlay is open (no selection change)', () => {
        mocks.commandPaletteMock.isOverlayOpen.mockReturnValue(true);
        keyDown('ArrowDown');
        expect(comp.selectedTaskIndex()).toBeNull();
    });

    it('handles ArrowDown normally when no overlay is open', () => {
        mocks.commandPaletteMock.isOverlayOpen.mockReturnValue(false);
        keyDown('ArrowDown');
        expect(comp.selectedTaskIndex()).toBe(0);
    });
});

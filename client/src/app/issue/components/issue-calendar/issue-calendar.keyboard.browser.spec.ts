import { describe, it, expect, beforeEach } from 'vitest';
import { createCalendarFixture } from './calendar-testbed.helper';

describe('IssueCalendarComponent keyboard navigation (TestBed)', () => {
    let fixture: any;
    let comp: any;
    let mocks: any;

    beforeEach(async () => {
        localStorage.clear();
        const result = await createCalendarFixture();
        fixture = result.fixture;
        comp = result.comp;
        mocks = result.mocks;
    });

    function keyDown(key: string, target: EventTarget | null = null) {
        const event = new KeyboardEvent('keydown', { key, bubbles: true });
        if (target) Object.defineProperty(event, 'target', { value: target });
        fixture.nativeElement.dispatchEvent(event);
        fixture.detectChanges();
    }

    function getApi() {
        return comp.calendarRef().getApi();
    }

    // =========================================================================
    // Gate / editable target
    // =========================================================================

    it('ignores all keys while the command palette overlay is open', () => {
        mocks.commandPaletteMock.isOverlayOpen.mockReturnValue(true);
        keyDown('ArrowLeft');
        expect(getApi().prev).not.toHaveBeenCalled();
    });

    it('ignores keys when the event target is an input element', () => {
        const input = document.createElement('input');
        keyDown('t', input);
        expect(getApi().today).not.toHaveBeenCalled();
    });

    it('ignores keys when the event target is a textarea', () => {
        const textarea = document.createElement('textarea');
        keyDown('ArrowRight', textarea);
        expect(getApi().next).not.toHaveBeenCalled();
    });

    // =========================================================================
    // t / Home (today)
    // =========================================================================

    it('t navigates to today', () => {
        keyDown('t');
        expect(getApi().today).toHaveBeenCalled();
    });

    it('Home navigates to today', () => {
        keyDown('Home');
        expect(getApi().today).toHaveBeenCalled();
    });

    // =========================================================================
    // ArrowLeft / ArrowRight
    // =========================================================================

    it('ArrowLeft navigates to previous period', () => {
        keyDown('ArrowLeft');
        expect(getApi().prev).toHaveBeenCalled();
    });

    it('ArrowRight navigates to next period', () => {
        keyDown('ArrowRight');
        expect(getApi().next).toHaveBeenCalled();
    });

    // =========================================================================
    // + / = / - (zoom)
    // =========================================================================

    it('+ zooms in (month → week)', () => {
        comp.currentView = 'dayGridMonth';
        keyDown('+');
        expect(getApi().changeView).toHaveBeenCalledWith('timeGridWeek');
    });

    it('= zooms in (month → week)', () => {
        comp.currentView = 'dayGridMonth';
        keyDown('=');
        expect(getApi().changeView).toHaveBeenCalledWith('timeGridWeek');
    });

    it('- zooms out (week → month)', () => {
        comp.currentView = 'timeGridWeek';
        keyDown('-');
        expect(getApi().changeView).toHaveBeenCalledWith('dayGridMonth');
    });

    it('+ on finest level (day) is no-op', () => {
        comp.currentView = 'timeGridDay';
        getApi().changeView.mockClear();
        keyDown('+');
        expect(getApi().changeView).not.toHaveBeenCalled();
    });

    it('- on coarsest level (month) is no-op', () => {
        comp.currentView = 'dayGridMonth';
        getApi().changeView.mockClear();
        keyDown('-');
        expect(getApi().changeView).not.toHaveBeenCalled();
    });

    // =========================================================================
    // Unmapped keys
    // =========================================================================

    it('unmapped key (e.g. "x") does nothing and does not throw', () => {
        getApi().today.mockClear();
        expect(() => keyDown('x')).not.toThrow();
        expect(getApi().today).not.toHaveBeenCalled();
    });
});

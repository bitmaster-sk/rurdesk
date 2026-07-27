import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createTableFixture } from './table-testbed.helper';

describe('IssueTableComponent.onRelationModeChange (TestBed)', () => {
    let comp: any;
    let mocks: any;

    beforeEach(async () => {
        localStorage.clear();
        const result = await createTableFixture();
        comp = result.comp;
        mocks = result.mocks;
    });

    it('sets isRelationMode and persists it to localStorage', () => {
        comp.onRelationModeChange(true);
        expect(comp.isRelationMode()).toBe(true);
        expect(localStorage.getItem('issue-relation-mode')).toBe('true');
    });

    it('disabling clears expanded issue set', () => {
        comp.idsExtendedIssue.set(new Set([1, 2]));
        comp.onRelationModeChange(false);
        expect(comp.idsExtendedIssue().size).toBe(0);
    });

    it('enabling does not clear expanded set', () => {
        const existing = new Set([1, 2]);
        comp.idsExtendedIssue.set(existing);
        comp.onRelationModeChange(true);
        expect(comp.idsExtendedIssue()).toBe(existing);
    });
});

describe('IssueTableComponent.onAskLagChange (TestBed)', () => {
    let comp: any;

    beforeEach(async () => {
        localStorage.clear();
        const result = await createTableFixture();
        comp = result.comp;
    });

    it('sets isAskLag and persists to localStorage', () => {
        comp.onAskLagChange(false);
        expect(comp.isAskLag()).toBe(false);
        expect(localStorage.getItem('issue-relation-ask-lag')).toBe('false');
    });

    it('enabling persists "true"', () => {
        comp.isAskLag.set(false);
        comp.onAskLagChange(true);
        expect(comp.isAskLag()).toBe(true);
        expect(localStorage.getItem('issue-relation-ask-lag')).toBe('true');
    });
});

describe('IssueTableComponent.onToggleRelations (TestBed)', () => {
    let comp: any;
    let mocks: any;

    beforeEach(async () => {
        localStorage.clear();
        const result = await createTableFixture();
        comp = result.comp;
        mocks = result.mocks;
    });

    it('expands: adds to idsExtendedIssue and lazy-loads relations on first expand', () => {
        comp.relationsLoaded = new Set();
        comp.onToggleRelations(5);
        expect(mocks.issueTableServiceMock.loadRelationsFor).toHaveBeenCalledWith(10, 5);
    });

    it('collapse: removes from idsExtendedIssue, does not load relations', () => {
        mocks.issueTableServiceMock.loadRelationsFor.mockClear();
        comp.idsExtendedIssue.set(new Set([5]));
        comp.relationsLoaded = new Set();
        comp.onToggleRelations(5);
        expect(mocks.issueTableServiceMock.loadRelationsFor).not.toHaveBeenCalled();
    });

    it('re-expand: does not lazy-load again (already loaded)', () => {
        mocks.issueTableServiceMock.loadRelationsFor.mockClear();
        comp.relationsLoaded = new Set([5]);
        comp.onToggleRelations(5);
        expect(mocks.issueTableServiceMock.loadRelationsFor).not.toHaveBeenCalled();
    });
});

describe('IssueTableComponent.markPulsed (TestBed)', () => {
    let comp: any;

    beforeEach(async () => {
        localStorage.clear();
        vi.useFakeTimers();
        const result = await createTableFixture();
        comp = result.comp;
    });

    afterEach(() => {
        vi.useRealTimers();
    });

    it('adds idIssuePublic to pulsedIds', () => {
        comp.markPulsed(42);
        expect(comp.pulsedIds().has(42)).toBe(true);
    });

    it('null idIssuePublic: no-op', () => {
        const before = comp.pulsedIds().size;
        comp.markPulsed(null);
        expect(comp.pulsedIds().size).toBe(before);
    });

    it('undefined idIssuePublic: no-op', () => {
        const before = comp.pulsedIds().size;
        comp.markPulsed(undefined);
        expect(comp.pulsedIds().size).toBe(before);
    });

    it('expires after 2500ms (removes from pulsedIds)', () => {
        comp.markPulsed(42);
        expect(comp.pulsedIds().has(42)).toBe(true);
        vi.advanceTimersByTime(2500);
        expect(comp.pulsedIds().has(42)).toBe(false);
    });

    it('already pulsing: calls restartRowPulse', () => {
        comp.pulsedIds.set(new Set([42]));
        const restartSpy = vi.spyOn(comp, 'restartRowPulse');
        comp.markPulsed(42);
        expect(restartSpy).toHaveBeenCalledWith(42);
    });

    it('not yet pulsing: does not call restartRowPulse', () => {
        comp.pulsedIds.set(new Set());
        const restartSpy = vi.spyOn(comp, 'restartRowPulse');
        comp.markPulsed(42);
        expect(restartSpy).not.toHaveBeenCalled();
    });

    it('clears previous timer for same id when re-pulsed', () => {
        const clearTimeoutSpy = vi.spyOn(globalThis, 'clearTimeout');
        const firstTimer = setTimeout(() => {}, 9999);
        comp.pulseTimers = new Map([[42, firstTimer]]);
        comp.pulsedIds.set(new Set([42]));
        comp.restartRowPulse = vi.fn();
        comp.markPulsed(42);
        expect(clearTimeoutSpy).toHaveBeenCalledWith(firstTimer);
        clearTimeoutSpy.mockRestore();
    });
});

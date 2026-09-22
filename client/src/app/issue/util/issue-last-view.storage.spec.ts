import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { IssueLastViewStorage } from './issue-last-view.storage';
import { IssueViewMode } from '../constants/issue-view-modes.enum';

const storage = new Map<string, string>();
vi.stubGlobal('localStorage', {
    getItem: (key: string) => storage.get(key) ?? null,
    setItem: (key: string, value: string) => storage.set(key, value),
    removeItem: (key: string) => storage.delete(key),
    clear: () => storage.clear()
});

describe('IssueLastViewStorage', () => {
    beforeEach(() => storage.clear());
    afterEach(() => storage.clear());

    it('saves and loads a valid view mode per project', () => {
        IssueLastViewStorage.save(7, IssueViewMode.KANBAN);
        expect(IssueLastViewStorage.load(7)).toBe(IssueViewMode.KANBAN);
    });

    it('returns null when no value is stored', () => {
        expect(IssueLastViewStorage.load(7)).toBeNull();
    });

    it('returns null for an unknown value', () => {
        localStorage.setItem('rurdesk.issue.lastView.7', 'unknown');
        expect(IssueLastViewStorage.load(7)).toBeNull();
    });

    it('overwrites the previous value for the same project', () => {
        IssueLastViewStorage.save(7, IssueViewMode.CALENDAR);
        IssueLastViewStorage.save(7, IssueViewMode.GANTT);
        expect(IssueLastViewStorage.load(7)).toBe(IssueViewMode.GANTT);
    });

    it('keeps values isolated by project id', () => {
        IssueLastViewStorage.save(7, IssueViewMode.TABLE);
        IssueLastViewStorage.save(8, IssueViewMode.KANBAN);
        expect(IssueLastViewStorage.load(7)).toBe(IssueViewMode.TABLE);
        expect(IssueLastViewStorage.load(8)).toBe(IssueViewMode.KANBAN);
    });
});

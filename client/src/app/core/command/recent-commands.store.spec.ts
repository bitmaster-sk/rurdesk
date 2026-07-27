// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from 'vitest';
import { RecentCommandsStore } from './recent-commands.store';

describe('RecentCommandsStore', () => {
    beforeEach(() => localStorage.clear());
    it('starts empty', () => expect(new RecentCommandsStore().recentIds()).toEqual([]));
    it('pushes most-recent-first and dedups', () => {
        const s = new RecentCommandsStore();
        s.push('a');
        s.push('b');
        s.push('a');
        expect(s.recentIds()).toEqual(['a', 'b']);
    });
    it('caps at 8', () => {
        const s = new RecentCommandsStore();
        for (const id of ['1', '2', '3', '4', '5', '6', '7', '8', '9']) s.push(id);
        expect(s.recentIds()).toHaveLength(8);
        expect(s.recentIds()[0]).toBe('9');
    });
    it('persists across instances', () => {
        new RecentCommandsStore().push('x');
        expect(new RecentCommandsStore().recentIds()).toEqual(['x']);
    });
});

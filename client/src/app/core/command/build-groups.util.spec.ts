import { describe, it, expect } from 'vitest';
import { buildGroups } from './build-groups.util';
import { Command } from './command.model';

const cmd = (over: Partial<Command>): Command => ({
    id: 'x',
    title: 'X',
    group: 'G',
    icon: 'i',
    modes: ['commands'],
    run: () => {},
    ...over
});

describe('buildGroups', () => {
    it('keeps only commands whose modes include the active prefixed mode', () => {
        const groups = buildGroups(
            [
                cmd({ id: 'a', title: 'Assign', modes: ['commands'] }),
                cmd({ id: 'n', title: 'Board', modes: ['navigation'] })
            ],
            'commands',
            '',
            []
        );
        expect(groups.flatMap(g => g.items.map(i => i.id))).toEqual(['a']);
    });
    it('filters by fuzzy query over title/subtitle/keywords', () => {
        const groups = buildGroups(
            [cmd({ id: 'a', title: 'Assign to me' }), cmd({ id: 'b', title: 'Set status' })],
            'commands',
            'assign',
            []
        );
        expect(groups.flatMap(g => g.items.map(i => i.id))).toEqual(['a']);
    });
    it('ranks recent commands first on an EMPTY query (browse state)', () => {
        const groups = buildGroups(
            [cmd({ id: 'a', title: 'Alpha' }), cmd({ id: 'b', title: 'Bravo' })],
            'commands',
            '',
            ['b']
        );
        expect(groups.flatMap(g => g.items.map(i => i.id))).toEqual(['b', 'a']);
    });
    it('on a NON-empty query a strong match beats a recent-but-weak match', () => {
        // 'b' is recent but only weakly matches 'assign'; 'a' matches exactly → 'a' wins.
        const groups = buildGroups(
            [cmd({ id: 'a', title: 'Assign' }), cmd({ id: 'b', title: 'Archive scan signal' })],
            'commands',
            'assign',
            ['b']
        );
        expect(groups.flatMap(g => g.items.map(i => i.id))[0]).toBe('a');
    });
    it('prefers an exact-token match (id via keywords) over a scattered fuzzy match', () => {
        // Mirrors real buildIssueJumpCommands output: title '#428 Login' + keywords '428' (Task 8).
        const groups = buildGroups(
            [
                cmd({ id: 'fuzzy', title: '4 apples 2 pears 8 plums', modes: ['issues'] }),
                cmd({ id: 'exact', title: '#428 Login', keywords: '428', modes: ['issues'] })
            ],
            'issues',
            '428',
            []
        );
        expect(groups.flatMap(g => g.items.map(i => i.id))[0]).toBe('exact');
    });
    it('matches the #-prefixed token when the user types the full #id', () => {
        const groups = buildGroups(
            [
                cmd({ id: 'other', title: '#4281 Something', keywords: '4281', modes: ['issues'] }),
                cmd({ id: 'exact', title: '#428 Login', keywords: '428', modes: ['issues'] })
            ],
            'issues',
            '#428',
            []
        );
        // '#428' is an exact whitespace-token of '#428 Login' but only a substring of '#4281 …'
        expect(groups.flatMap(g => g.items.map(i => i.id))[0]).toBe('exact');
    });
    it('attaches highlight segments', () => {
        const groups = buildGroups([cmd({ id: 'a', title: 'Assign' })], 'commands', 'as', []);
        expect(groups[0].items[0].highlight[0]).toEqual({ text: 'As', hit: true });
    });
    it('groups under headings preserving first-seen order', () => {
        const groups = buildGroups(
            [
                cmd({ id: 'a', title: 'A', group: 'People' }),
                cmd({ id: 'b', title: 'B', group: 'Actions' }),
                cmd({ id: 'c', title: 'C', group: 'People' })
            ],
            'commands',
            '',
            []
        );
        expect(groups.map(g => g.heading)).toEqual(['People', 'Actions']);
        expect(groups[0].items.map(i => i.id)).toEqual(['a', 'c']);
    });
});

import { describe, it, expect, vi } from 'vitest';
import { buildNavigationCommands, buildGlobalCommands } from './navigation.commands';

const ctx = (idProject: number | null) => ({ idProject, issue: null });
const t = (k: string) => k; // identity translator → title === key; assertions target stable ids

describe('buildNavigationCommands', () => {
    const acl = { canOpenSettings: true };
    it('offers the six project destinations when a project is in context', () => {
        const ids = buildNavigationCommands(ctx(3), [], () => {}, acl, t).map(c => c.id);
        expect(ids).toEqual(
            expect.arrayContaining([
                'nav.overview',
                'nav.table',
                'nav.kanban',
                'nav.calendar',
                'nav.gantt',
                'nav.settings'
            ])
        );
    });
    it('routes the Board destination to the kanban view', () => {
        const nav = vi.fn();
        buildNavigationCommands(ctx(3), [], nav, acl, t)
            .find(c => c.id === 'nav.kanban')!
            .run();
        expect(nav).toHaveBeenCalledWith(['/project', 3, 'issue', 'view', 'kanban']);
    });
    it('navigation destinations live in navigation mode, NOT commands mode', () => {
        const board = buildNavigationCommands(ctx(3), [], () => {}, acl, t).find(
            c => c.id === 'nav.kanban'
        )!;
        expect(board.modes).toEqual(expect.arrayContaining(['all', 'navigation']));
        expect(board.modes).not.toContain('commands');
    });
    it('omits settings when the user cannot open settings', () => {
        const ids = buildNavigationCommands(
            ctx(3),
            [],
            () => {},
            { canOpenSettings: false },
            t
        ).map(c => c.id);
        expect(ids).not.toContain('nav.settings');
    });
    it('lists switch-project commands with the untranslated project name (data)', () => {
        const switchers = buildNavigationCommands(
            ctx(3),
            [{ idProject: 7, name: 'Website' }],
            () => {},
            acl,
            t
        ).filter(c => c.id.startsWith('nav.switch.'));
        expect(switchers.map(c => c.title)).toEqual(['Website']);
    });
    it('hides project destinations when there is no project', () => {
        expect(
            buildNavigationCommands(ctx(null), [], () => {}, acl, t).map(c => c.id)
        ).not.toContain('nav.kanban');
    });
});

describe('buildGlobalCommands', () => {
    const actions = () => ({ createIssue: vi.fn(), openShortcuts: vi.fn(), signOut: vi.fn() });
    it('offers create/shortcuts/sign-out and wires actions when create is allowed', () => {
        const a = actions();
        const cmds = buildGlobalCommands(ctx(null), a, { canCreateIssue: true }, t);
        expect(cmds.map(c => c.id)).toEqual([
            'global.create',
            'global.shortcuts',
            'global.signout'
        ]);
        cmds[0].run();
        expect(a.createIssue).toHaveBeenCalled();
    });
    it('omits create when the user cannot create', () => {
        const cmds = buildGlobalCommands(ctx(null), actions(), { canCreateIssue: false }, t);
        expect(cmds.map(c => c.id)).toEqual(['global.shortcuts', 'global.signout']);
    });
});

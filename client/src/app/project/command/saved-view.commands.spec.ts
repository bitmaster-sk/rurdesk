import { describe, expect, it, vi } from 'vitest';
import { CommandContext } from '../../core/command/command.model';
import { SavedView } from '../model/saved-view.model';
import { IssueViewMode } from '../../issue/constants/issue-view-modes.enum';
import { buildSavedViewCommands } from './saved-view.commands';

const t = (key: string): string => key;

function ctx(idProject: number | null): CommandContext {
    return { idProject, issue: null };
}

function view(over: Partial<SavedView> = {}): SavedView {
    return {
        idSavedView: 7,
        idProject: 1,
        name: 'My bugs',
        viewType: IssueViewMode.TABLE,
        isShared: false,
        createBy: 1,
        updateAt: '2026-08-01T00:00:00Z',
        config: { v: 1 },
        ...over
    };
}

describe('buildSavedViewCommands', () => {
    it('builds one command per view, titled by name', () => {
        const commands = buildSavedViewCommands(
            ctx(1),
            [view({ idSavedView: 1, name: 'Alpha' }), view({ idSavedView: 2, name: 'Beta' })],
            vi.fn(),
            t
        );

        expect(commands.map(command => command.title)).toEqual(['Alpha', 'Beta']);
        expect(commands.map(command => command.id)).toEqual(['saved-view:1', 'saved-view:2']);
    });

    it('surfaces views in the navigation list under their own heading', () => {
        const [command] = buildSavedViewCommands(ctx(1), [view()], vi.fn(), t);

        expect(command.modes).toContain('navigation');
        expect(command.modes).toContain('all');
        expect(command.group).toBe('COMMAND.GROUP.VIEWS');
        // Merging into NAV would sort views among the destinations and bury them.
        expect(command.group).not.toBe('COMMAND.GROUP.NAV');
        expect(command.icon).toBe('bookmark');
    });

    it('makes the view type searchable as a keyword', () => {
        const [command] = buildSavedViewCommands(
            ctx(1),
            [view({ viewType: IssueViewMode.KANBAN })],
            vi.fn(),
            t
        );

        expect(command.keywords).toBe('kanban');
    });

    // The root-scoped provider cannot reach the apply service, so `run` only navigates and
    // the issue page's ?view= handler applies on arrival.
    it('navigates to the view route with the view id as a query param', () => {
        const nav = vi.fn();
        const [command] = buildSavedViewCommands(
            ctx(1),
            [view({ idSavedView: 9, viewType: IssueViewMode.GANTT })],
            nav,
            t
        );

        command.run();

        expect(nav).toHaveBeenCalledWith(['/project', 1, 'issue', 'view', 'gantt'], { view: 9 });
    });

    it('offers nothing outside a project', () => {
        expect(buildSavedViewCommands(ctx(null), [view()], vi.fn(), t)).toEqual([]);
    });

    it('offers nothing when there are no views', () => {
        expect(buildSavedViewCommands(ctx(1), [], vi.fn(), t)).toEqual([]);
    });
});

import { Command, CommandContext, Translator } from '../../core/command/command.model';
import { SavedView } from '../model/saved-view.model';

export function buildSavedViewCommands(
    ctx: CommandContext,
    views: SavedView[],
    nav: (path: unknown[], queryParams: Record<string, unknown>) => void,
    t: Translator
): Command[] {
    const idProject = ctx.idProject;
    if (idProject == null) {
        return [];
    }
    return views.map(view => ({
        id: `saved-view:${view.idSavedView}`,
        title: view.name,
        group: t('COMMAND.GROUP.VIEWS'),
        icon: 'bookmark',
        keywords: view.viewType,
        modes: ['all', 'navigation'],
        run: () =>
            nav(['/project', idProject, 'issue', 'view', view.viewType], {
                view: view.idSavedView
            })
    }));
}

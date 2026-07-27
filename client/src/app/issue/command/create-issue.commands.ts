import { Command, CommandContext, Translator } from '../../core/command/command.model';

export function buildCreateFromQuery(
    query: string,
    ctx: CommandContext,
    canCreateIssue: boolean,
    create: (title: string) => void,
    t: Translator
): Command | null {
    if (!query || ctx.idProject == null || !canCreateIssue) return null;
    return {
        id: 'issue.create',
        title: t('COMMAND.ACTION.CREATE_WITH_TITLE', { title: query }),
        subtitle: t('COMMAND.ACTION.CREATE_HINT'),
        group: t('COMMAND.GROUP.CREATE'),
        icon: 'plus',
        modes: ['all', 'issues'],
        run: () => create(query)
    };
}

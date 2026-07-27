import { Command, CommandContext, Translator } from '../../core/command/command.model';
import { Issue } from '../model/issue.model';

export function buildIssueJumpCommands(
    ctx: CommandContext,
    issues: Issue[],
    nav: (path: unknown[]) => void,
    t: Translator
): Command[] {
    if (ctx.idProject == null) return [];
    return issues.map(issue => ({
        id: `issue.jump.${issue.idIssuePublic}`,
        title: `#${issue.idIssuePublic} ${issue.title}`,
        keywords: String(issue.idIssuePublic),
        completion: `#${issue.idIssuePublic}`,
        group: t('COMMAND.GROUP.ISSUES'),
        icon: 'search',
        modes: ['all', 'issues'],
        run: () => nav(['/project', ctx.idProject, 'issue', issue.idIssuePublic])
    }));
}

import { Command, CommandContext, Translator } from '../../core/command/command.model';
import { Issue } from '../model/issue.model';
import { IssueState } from '../../state/model/issue-state.model';
import { IssueSeverity } from '../../severity/model/issue-severity.model';
import { User } from '../../auth/model/user.model';

interface ActionData {
    states: IssueState[];
    severities: IssueSeverity[];
    users: User[];
    currentUserId: number | null;
}

export function buildIssueActionCommands(
    ctx: CommandContext,
    data: ActionData,
    patch: (over: Partial<Issue>) => void,
    t: Translator
): Command[] {
    if (!ctx.issue || ctx.idProject == null) return [];
    const commands: Command[] = [];

    for (const state of data.states.filter(s => s.idProject === ctx.idProject)) {
        commands.push({
            id: `issue.state.${state.idState}`,
            title: t('COMMAND.ISSUE.SET_STATE', { name: state.name }),
            group: t('COMMAND.GROUP.STATE'),
            icon: 'circle-check',
            modes: ['commands'],
            run: () => patch({ idState: state.idState })
        });
    }
    if (data.currentUserId != null) {
        const me = data.currentUserId;
        commands.push({
            id: 'issue.assign.me',
            title: t('COMMAND.ISSUE.ASSIGN_ME'),
            group: t('COMMAND.GROUP.ASSIGN'),
            icon: 'user',
            modes: ['commands'],
            run: () => patch({ assignedTo: me })
        });
    }
    for (const user of data.users) {
        commands.push({
            id: `issue.assign.${user.idUser}`,
            title: t('COMMAND.ISSUE.ASSIGN_TO', { name: user.name }),
            group: t('COMMAND.GROUP.ASSIGN'),
            icon: 'user',
            modes: ['commands'],
            run: () => patch({ assignedTo: user.idUser })
        });
    }
    for (const sev of data.severities.filter(s => s.idProject === ctx.idProject)) {
        commands.push({
            id: `issue.sev.${sev.idSeverity}`,
            title: t('COMMAND.ISSUE.SET_SEVERITY', { name: sev.title }),
            group: t('COMMAND.GROUP.SEVERITY'),
            icon: 'flag',
            modes: ['commands'],
            run: () => patch({ idSeverity: sev.idSeverity })
        });
    }
    return commands;
}

/** "Clone task" — offered on an open task when the user may create tasks. */
export function buildIssueCloneCommand(
    ctx: CommandContext,
    canCreateIssue: boolean,
    clone: () => void,
    t: Translator
): Command | null {
    if (!ctx.issue || ctx.idProject == null || !canCreateIssue) return null;
    return {
        id: 'issue.clone',
        title: t('COMMAND.ISSUE.CLONE'),
        group: t('COMMAND.GROUP.CREATE'),
        icon: 'copy',
        modes: ['all', 'commands'],
        run: clone
    };
}

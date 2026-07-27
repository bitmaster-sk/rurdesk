import { Command, CommandContext, Translator } from '../../core/command/command.model';
import { IssueViewMode } from '../../issue/constants/issue-view-modes.enum';

// Icons + labels mirror the project left-menu view links (project-layout.component.html).
const VIEWS: { titleKey: string; icon: string; mode: IssueViewMode }[] = [
    { titleKey: 'COMMAND.NAV.TABLE', icon: 'table', mode: IssueViewMode.TABLE },
    { titleKey: 'COMMAND.NAV.BOARD', icon: 'layout-columns', mode: IssueViewMode.KANBAN },
    { titleKey: 'COMMAND.NAV.CALENDAR', icon: 'calendar', mode: IssueViewMode.CALENDAR },
    { titleKey: 'COMMAND.NAV.GANTT', icon: 'chart-column', mode: IssueViewMode.GANTT }
];

export function buildNavigationCommands(
    ctx: CommandContext,
    projects: { idProject: number; name: string }[],
    nav: (path: unknown[]) => void,
    acl: { canOpenSettings: boolean },
    t: Translator
): Command[] {
    const commands: Command[] = [];
    const id = ctx.idProject;
    if (id != null) {
        // Destinations live in `all` + `navigation` only — NOT `commands`. `>` is for actions
        // (issue ops, global commands); navigation has its own `/` prefix, so surfacing nav rows
        // under `>` was noise.
        const dest = (cid: string, titleKey: string, icon: string, path: unknown[]): Command => ({
            id: cid,
            title: t(titleKey),
            icon,
            group: t('COMMAND.GROUP.NAV'),
            modes: ['all', 'navigation'],
            run: () => nav(path)
        });
        commands.push(
            dest('nav.overview', 'COMMAND.NAV.OVERVIEW', 'home', ['/project', id, 'view'])
        );
        for (const v of VIEWS)
            commands.push(
                dest(`nav.${v.mode}`, v.titleKey, v.icon, ['/project', id, 'issue', 'view', v.mode])
            );
        if (acl.canOpenSettings)
            commands.push(
                dest('nav.settings', 'COMMAND.NAV.SETTINGS', 'settings', [
                    '/project',
                    id,
                    'settings'
                ])
            );
    }
    for (const p of projects) {
        commands.push({
            id: `nav.switch.${p.idProject}`,
            title: p.name,
            subtitle: t('COMMAND.NAV.SWITCH_PROJECT'),
            group: t('COMMAND.GROUP.PROJECTS'),
            icon: 'layout-board',
            modes: ['all', 'navigation'],
            run: () => nav(['/project', p.idProject, 'view'])
        });
    }
    return commands;
}

export function buildGlobalCommands(
    _ctx: CommandContext,
    actions: { createIssue: () => void; openShortcuts: () => void; signOut: () => void },
    acl: { canCreateIssue: boolean },
    t: Translator
): Command[] {
    const g = t('COMMAND.GROUP.COMMANDS');
    const commands: Command[] = [];
    if (acl.canCreateIssue) {
        // Grouped under "Create" (with clone + create-from-query), not "Commands", so all
        // task-creation actions sit together.
        commands.push({
            id: 'global.create',
            title: t('COMMAND.ACTION.CREATE_ISSUE'),
            group: t('COMMAND.GROUP.CREATE'),
            icon: 'plus',
            modes: ['all', 'commands'],
            run: actions.createIssue
        });
    }
    commands.push({
        id: 'global.shortcuts',
        title: t('COMMAND.ACTION.SHORTCUTS'),
        group: g,
        icon: 'keyboard',
        modes: ['all', 'commands'],
        run: actions.openShortcuts
    });
    commands.push({
        id: 'global.signout',
        title: t('COMMAND.ACTION.SIGN_OUT'),
        group: g,
        icon: 'logout',
        modes: ['commands'],
        run: actions.signOut
    });
    return commands;
}

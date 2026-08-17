import { Injectable, inject } from '@angular/core';
import { Router } from '@angular/router';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { Observable } from 'rxjs';
import { tap } from 'rxjs/operators';
import {
    Command,
    CommandContext,
    CommandProvider,
    Translator
} from '../../core/command/command.model';
import { buildNavigationCommands, buildGlobalCommands } from './navigation.commands';
import { ProjectService } from '../project.service';
import { Project } from '../model/project.model';
import { AclStore } from '../store/acl.store';
import { CommandPaletteService } from '../../core/command/command-palette.service';
import { UserService } from '../../auth/user.service';

@Injectable({ providedIn: 'root' })
export class NavigationCommandProvider implements CommandProvider {
    private readonly router = inject(Router);
    private readonly projectService = inject(ProjectService);
    private readonly acl = inject(AclStore);
    private readonly palette = inject(CommandPaletteService);
    private readonly userService = inject(UserService);
    private readonly i18n = inject(I18nService);
    private readonly t: Translator = (k, p) => this.i18n.instant(k, p);

    private projects: { idProject: number; name: string }[] = [];

    public prime(_ctx: CommandContext): Observable<unknown> {
        return this.projectService.loadProjects().pipe(
            tap((list: Project[]) => {
                this.projects = list.map(p => ({ idProject: p.idProject!, name: p.name }));
            })
        );
    }

    public getCommands(ctx: CommandContext): Command[] {
        const others = this.projects.filter(p => p.idProject !== ctx.idProject);
        const nav = buildNavigationCommands(
            ctx,
            others,
            p => this.router.navigate(p as unknown[]),
            { canOpenSettings: this.acl.canUpdateProject() },
            this.t
        );
        const global = buildGlobalCommands(
            ctx,
            {
                // `0` is the app's new-issue sentinel (same target as the Tasks page "+" button)
                // → opens the blank issue form, not the table listing.
                createIssue: () => {
                    if (ctx.idProject != null)
                        this.router.navigate(['/project', ctx.idProject, 'issue', 0]);
                },
                openShortcuts: () => this.palette.openHelp(),
                signOut: () => this.signOut()
            },
            { canCreateIssue: this.acl.canCreateIssue() },
            this.t
        );
        return [...nav, ...global];
    }

    private signOut(): void {
        const done = (): void => {
            this.userService.deleteAuthLocal();
            this.router.navigate(['/login']);
        };
        this.userService.logout().subscribe({ next: done, error: done });
    }
}

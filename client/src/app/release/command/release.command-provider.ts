import { Injectable, inject } from '@angular/core';
import { Router } from '@angular/router';
import { Observable, of } from 'rxjs';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { Command, CommandContext, CommandProvider } from '../../core/command/command.model';
import { AclStore } from '../../project/store/acl.store';

@Injectable({ providedIn: 'root' })
export class ReleaseCommandProvider implements CommandProvider {
    private readonly router = inject(Router);
    private readonly acl = inject(AclStore);
    private readonly i18n = inject(I18nService);

    public prime(_ctx: CommandContext): Observable<unknown> {
        return of(null);
    }

    public getCommands(ctx: CommandContext): Command[] {
        const idProject = ctx.idProject;
        if (idProject === null) {
            return [];
        }
        const group = this.i18n.instant('RELEASE.COMMAND.GROUP');
        const commands: Command[] = [
            {
                id: 'release.open',
                title: this.i18n.instant('RELEASE.COMMAND.OPEN'),
                group,
                icon: 'rocket',
                keywords: 'release version changelog',
                modes: ['all', 'navigation'],
                run: () => void this.router.navigate(['/project', idProject, 'release'])
            }
        ];
        if (this.acl.canUpdateProject()) {
            commands.push({
                id: 'release.new',
                title: this.i18n.instant('RELEASE.COMMAND.NEW'),
                group,
                icon: 'rocket',
                keywords: 'release version publish changelog notes',
                modes: ['all', 'commands'],
                run: () => void this.router.navigate(['/project', idProject, 'release', 'new'])
            });
        }
        return commands;
    }
}

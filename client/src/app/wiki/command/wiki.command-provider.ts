import { Injectable, inject } from '@angular/core';
import { Router } from '@angular/router';
import { Observable, of } from 'rxjs';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { Command, CommandContext, CommandProvider } from '../../core/command/command.model';
import { WikiSpaceKind } from '../constants/wiki-space-kind.enum';
import { WikiTreeStore } from '../store/wiki-tree.store';

@Injectable({ providedIn: 'root' })
export class WikiCommandProvider implements CommandProvider {
    private readonly router = inject(Router);
    private readonly store = inject(WikiTreeStore);
    private readonly i18n = inject(I18nService);

    public prime(ctx: CommandContext): Observable<unknown> {
        if (ctx.idProject === null) {
            return of(null);
        }
        if (this.store.idProject() === ctx.idProject && this.store.tree()) {
            return of(null);
        }
        return this.store.load$(ctx.idProject);
    }

    public getCommands(ctx: CommandContext): Command[] {
        const idProject = ctx.idProject;
        if (idProject === null) {
            return [];
        }
        const group = this.i18n.instant('WIKI.COMMAND.GROUP');
        const commands: Command[] = [
            {
                id: 'wiki.open',
                title: this.i18n.instant('WIKI.COMMAND.OPEN'),
                group,
                icon: 'book-2',
                keywords: 'wiki docs knowledge',
                modes: ['all', 'navigation'],
                run: () => void this.router.navigate(['/project', idProject, 'wiki'])
            }
        ];
        if (this.store.idProject() === idProject && this.store.projectSpace()?.canEdit) {
            commands.push({
                id: 'wiki.new',
                title: this.i18n.instant('WIKI.COMMAND.NEW'),
                group,
                icon: 'plus',
                keywords: 'wiki page create',
                modes: ['all', 'commands'],
                run: () =>
                    void this.router.navigate(['/project', idProject, 'wiki', 'new'], {
                        queryParams: { space: WikiSpaceKind.Project }
                    })
            });
        }
        if (this.store.idProject() !== idProject) {
            return commands;
        }
        for (const node of this.store.tree()?.nodes ?? []) {
            const space = this.store.kindOfSpace(node.idSpace);
            commands.push({
                id: `wiki.page.${node.idPage}`,
                title: node.title,
                subtitle: this.i18n.instant(
                    space === WikiSpaceKind.Instance ? 'WIKI.SPACE.INSTANCE' : 'WIKI.SPACE.PROJECT'
                ),
                group,
                icon: 'file-text',
                keywords: `wiki ${node.slug}`,
                modes: ['all', 'navigation'],
                run: () =>
                    void this.router.navigate(['/project', idProject, 'wiki', space, node.slug])
            });
        }
        return commands;
    }
}

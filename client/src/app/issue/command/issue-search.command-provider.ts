import { Injectable, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { Router } from '@angular/router';
import { TranslateService } from '@ngx-translate/core';
import { Observable, of } from 'rxjs';
import { tap } from 'rxjs/operators';
import {
    Command,
    CommandContext,
    CommandProvider,
    Translator
} from '../../core/command/command.model';
import { buildIssueJumpCommands } from './issue-search.commands';
import { buildCreateFromQuery } from './create-issue.commands';
import { IssueService } from '../issue.service';
import { StateStore } from '../../state/store/state.store';
import { AclStore } from '../../project/store/acl.store';
import { Issue } from '../model/issue.model';

@Injectable({ providedIn: 'root' })
export class IssueSearchCommandProvider implements CommandProvider {
    private readonly router = inject(Router);
    private readonly issueService = inject(IssueService);
    private readonly acl = inject(AclStore);
    private readonly translate = inject(TranslateService);
    private readonly t: Translator = (k, p) => this.translate.instant(k, p);
    private readonly states = toSignal(inject(StateStore).states$, { initialValue: [] });

    /** Cache keyed by project — cleared/refetched on project switch so a stale project's
     *  jump links never leak into another project between switch and reload. */
    private cache: { idProject: number; issues: Issue[] } | null = null;

    public prime(ctx: CommandContext): Observable<unknown> {
        const idProject = ctx.idProject;
        if (idProject == null) return of(null);
        if (this.cache && this.cache.idProject !== idProject) this.cache = null;
        return this.issueService
            .loadIssues({ idProject, orderColumn: 'createAt', orderDirection: 'desc' })
            .pipe(
                tap((list: Issue[]) => {
                    this.cache = { idProject, issues: list };
                })
            );
    }

    public getCommands(ctx: CommandContext): Command[] {
        if (!this.cache || this.cache.idProject !== ctx.idProject) return [];
        return buildIssueJumpCommands(
            ctx,
            this.cache.issues,
            p => this.router.navigate(p as unknown[]),
            this.t
        );
    }

    public createFromQuery(query: string, ctx: CommandContext): Command | null {
        return buildCreateFromQuery(
            query,
            ctx,
            this.acl.canCreateIssue(),
            title => this.createIssue(title, ctx),
            this.t
        );
    }

    private createIssue(title: string, ctx: CommandContext): void {
        const idProject = ctx.idProject;
        if (idProject === null) {
            return;
        }
        // Default state = the project's START state (IssueState.start), fallback = lowest orderRank.
        // StateStore does NOT sort, so select explicitly — never rely on array position.
        const projectStates = this.states().filter(s => s.idProject === ctx.idProject);
        const defaultState =
            projectStates.find(s => s.start) ??
            [...projectStates].sort((a, b) => a.orderRank - b.orderRank)[0];
        this.issueService
            .insertIssue({
                idProject,
                title,
                description: title,
                idState: defaultState?.idState ?? null,
                idSeverity: null,
                tracked: 0
            })
            .subscribe(created =>
                this.router.navigate(['/project', ctx.idProject, 'issue', created.idIssuePublic])
            );
    }
}

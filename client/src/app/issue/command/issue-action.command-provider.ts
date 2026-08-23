import { Injectable, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { Router } from '@angular/router';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import {
    Command,
    CommandContext,
    CommandProvider,
    Translator
} from '../../core/command/command.model';
import { buildIssueActionCommands, buildIssueCloneCommand } from './issue-action.commands';
import { IssueService } from '../issue.service';
import { StateStore } from '../../state/store/state.store';
import { SeverityStore } from '../../severity/store/severity.store';
import { ProjectMemberStore } from '../../project/project-member.store';
import { AclStore } from '../../project/store/acl.store';
import { AuthStore } from '../../auth/store/auth.store';
import { NoticeService } from '../../shared/notice/notice.service';
import { Issue } from '../model/issue.model';

@Injectable({ providedIn: 'root' })
export class IssueActionCommandProvider implements CommandProvider {
    private readonly router = inject(Router);
    private readonly issueService = inject(IssueService);
    private readonly acl = inject(AclStore);
    private readonly authStore = inject(AuthStore);
    private readonly notice = inject(NoticeService);
    private readonly i18n = inject(I18nService);
    private readonly t: Translator = (k, p) => this.i18n.instant(k, p);
    private readonly states = toSignal(inject(StateStore).states$, { initialValue: [] });
    private readonly severities = toSignal(inject(SeverityStore).severities$, { initialValue: [] });
    private readonly users = toSignal(inject(ProjectMemberStore).users$, { initialValue: [] });

    public getCommands(ctx: CommandContext): Command[] {
        const commands: Command[] = [];
        if (this.acl.canUpdateIssue()) {
            commands.push(
                ...buildIssueActionCommands(
                    ctx,
                    {
                        states: this.states(),
                        severities: this.severities(),
                        users: this.users() ?? [],
                        currentUserId: this.authStore.user()?.idUser ?? null
                    },
                    (over: Partial<Issue>) => this.patch(ctx, over),
                    this.t
                )
            );
        }
        const clone = buildIssueCloneCommand(
            ctx,
            this.acl.canCreateIssue(),
            () => this.clone(ctx),
            this.t
        );
        if (clone) commands.push(clone);
        return commands;
    }

    private patch(ctx: CommandContext, over: Partial<Issue>): void {
        if (!ctx.issue) return;
        // Emit the saved issue locally so an open task detail refreshes at once (the server does
        // not echo the acting client's own change back over the socket).
        this.issueService
            .updateIssue({ ...ctx.issue, ...over })
            .subscribe(saved => this.notice.emitIssue(saved));
    }

    private clone(ctx: CommandContext): void {
        const src = ctx.issue;
        if (!src || ctx.idProject == null) return;
        this.issueService
            .insertIssue({
                idProject: ctx.idProject,
                title: this.t('ISSUE.COPY_SUFFIX', { title: src.title }),
                description: src.description,
                idState: src.idState,
                idSeverity: src.idSeverity,
                idIssueType: src.idIssueType,
                assignedTo: src.assignedTo,
                tracked: 0,
                estimated: src.estimated,
                scheduledAt: src.scheduledAt
            })
            .subscribe(created => {
                void this.router.navigate([
                    '/project',
                    ctx.idProject,
                    'issue',
                    created.idIssuePublic
                ]);
            });
    }
}

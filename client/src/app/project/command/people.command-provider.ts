import { Injectable, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { Router } from '@angular/router';
import { TranslateService } from '@ngx-translate/core';
import {
    Command,
    CommandContext,
    CommandProvider,
    Translator
} from '../../core/command/command.model';
import { buildPeopleCommands } from './people.commands';
import { ProjectMemberStore } from '../project-member.store';
import { AclStore } from '../store/acl.store';
import { IssueService } from '../../issue/issue.service';
import { NoticeService } from '../../shared/notice/notice.service';

@Injectable({ providedIn: 'root' })
export class PeopleCommandProvider implements CommandProvider {
    private readonly router = inject(Router);
    private readonly acl = inject(AclStore);
    private readonly issueService = inject(IssueService);
    private readonly notice = inject(NoticeService);
    private readonly translate = inject(TranslateService);
    private readonly t: Translator = (k, p) => this.translate.instant(k, p);
    private readonly users = toSignal(inject(ProjectMemberStore).users$, { initialValue: [] });

    public getCommands(ctx: CommandContext): Command[] {
        return buildPeopleCommands(
            ctx,
            this.users() ?? [],
            idUser => this.onPick(idUser, ctx),
            this.t
        );
    }

    private onPick(idUser: number, ctx: CommandContext): void {
        // On an issue detail, picking a person ASSIGNS them to the open issue (the natural
        // intent there). Off a detail (list views), fall back to opening the table.
        if (ctx.issue && this.acl.canUpdateIssue()) {
            // Emit the saved task so an open detail's assignee select refreshes immediately.
            this.issueService
                .updateIssue({ ...ctx.issue, assignedTo: idUser })
                .subscribe(saved => this.notice.emitIssue(saved));
            return;
        }
        if (ctx.idProject != null)
            this.router.navigate(['/project', ctx.idProject, 'issue', 'view', 'table']);
    }
}

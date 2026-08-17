import {
    ChangeDetectionStrategy,
    Component,
    computed,
    inject,
    OnDestroy,
    OnInit,
    signal,
    viewChild
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { Subscription } from 'rxjs';
import { UiMenuComponent } from 'src/app/ui/components/menu/menu.component';
import { UiMenuItem } from 'src/app/ui/components/menu/menu-item.model';
import { filter } from 'rxjs/operators';
import { User } from 'src/app/auth/model/user.model';
import { UserService } from 'src/app/auth/user.service';
import { MessageRecipientType } from 'src/app/message/constant/message-recipient-type.enum';
import { Message } from 'src/app/message/model/message.model';
import { MessageService } from 'src/app/message/message.service';
import { MessageKeyConverter } from 'src/app/message/converter/message-key.converter';
import { Project } from 'src/app/project/model/project.model';
import { ProjectService } from 'src/app/project/project.service';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { Team } from 'src/app/team/model/team.model';
import { TeamService } from 'src/app/team/team.service';
import { UserApi } from 'src/app/user/api/user.api.service';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';

@Component({
    selector: 'app-message-menu',
    templateUrl: './message-menu.component.html',
    styleUrls: ['./message-menu.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class MessageMenuComponent implements OnInit, OnDestroy {
    private readonly sMessage = inject(MessageService);
    private readonly sUser = inject(UserService);
    private readonly sTeam = inject(TeamService);
    private readonly userApi = inject(UserApi);
    private readonly sProject = inject(ProjectService);
    private readonly sNotice = inject(NoticeService);
    private readonly i18n = inject(I18nService);

    private readonly chatMenu = viewChild.required<UiMenuComponent>('chatMenu');

    private readonly _projects = signal<Project[]>([]);
    private readonly _teams = signal<Team[]>([]);
    private readonly _teammates = signal<User[]>([]);

    private readonly unread = toSignal(this.sMessage.Unread, {
        initialValue: new Map<string, Message[]>()
    });

    protected readonly chats = computed<UiMenuItem[]>(() => {
        const unreadMap = this.unread();
        const currentUserId = this.sUser.getUser().idUser;
        return [
            ...this.toProjectChatMenuItems(this._projects(), unreadMap),
            ...this.toTeamChatMenuItems(this._teams(), unreadMap),
            ...this.toTeammatesChatMenuItems(this._teammates(), unreadMap, currentUserId)
        ];
    });

    protected readonly messageCount = computed<number>(() => {
        let cnt = 0;
        this.unread().forEach(msgs => (cnt += msgs.length));
        return cnt;
    });

    protected readonly messageCountDisplay = computed<string>(() => {
        const count = this.messageCount();
        return count > 99 ? '99+' : count.toString();
    });

    private readonly subscriptions = new Subscription();

    public ngOnInit(): void {
        this.sMessage.loadUnreadMessages().subscribe();

        this.sProject.loadProjects().subscribe(projects => {
            this._projects.set(projects);
        });

        this.sTeam.loadMyTeams().subscribe(teams => {
            this._teams.set(teams);
        });

        this.userApi.loadUsers$().subscribe(users => {
            this._teammates.set(users.filter(u => !u.isBot));
        });

        this.subscriptions.add(
            this.sNotice.Message.pipe(
                filter(
                    notice =>
                        notice.payload.idMessageRecipientType !== MessageRecipientType.issue &&
                        notice.action !== NoticeAction.Update
                )
            ).subscribe(notice => this.sMessage.unreadPush([notice.payload]))
        );
    }

    public ngOnDestroy(): void {
        this.subscriptions.unsubscribe();
    }

    protected onChatClick(event: MouseEvent): void {
        this.chatMenu().toggle(event);
    }

    private toProjectChatMenuItems(
        projects: Project[],
        unreadMap: Map<string, Message[]>
    ): UiMenuItem[] {
        return [
            {
                label: this.i18n.instant('PROJECT.CHATS'),
                items: projects.map(p => this.toProjectChatMenuItem(p, unreadMap))
            }
        ];
    }

    private toProjectChatMenuItem(project: Project, unreadMap: Map<string, Message[]>): UiMenuItem {
        const key = MessageKeyConverter.toUnreadKey(
            project.idProject,
            null,
            MessageRecipientType.project
        );
        const count = unreadMap.get(key)?.length ?? 0;
        return {
            label: project.name,
            icon: 'messages',
            badge: count > 0 ? count.toString() : undefined,
            badgeSeverity: 'danger',
            routerLink: ['/message', project.idProject, MessageRecipientType.project, 'view']
        };
    }

    private toTeamChatMenuItems(teams: Team[], unreadMap: Map<string, Message[]>): UiMenuItem[] {
        return [
            {
                label: this.i18n.instant('TEAM.CHATS'),
                items: teams.map(t => this.toTeamChatMenuItem(t, unreadMap))
            }
        ];
    }

    private toTeamChatMenuItem(team: Team, unreadMap: Map<string, Message[]>): UiMenuItem {
        const key = MessageKeyConverter.toUnreadKey(team.idTeam, null, MessageRecipientType.team);
        const count = unreadMap.get(key)?.length ?? 0;
        return {
            label: team.name,
            icon: 'users',
            badge: count > 0 ? count.toString() : undefined,
            badgeSeverity: 'danger',
            routerLink: ['/message', team.idTeam, MessageRecipientType.team, 'view']
        };
    }

    private toTeammatesChatMenuItems(
        users: User[],
        unreadMap: Map<string, Message[]>,
        currentUserId: number
    ): UiMenuItem[] {
        return [
            {
                label: this.i18n.instant('DIRECT.CHATS'),
                items: users.map(u => this.toTeammatesChatMenuItem(u, unreadMap, currentUserId))
            }
        ];
    }

    private toTeammatesChatMenuItem(
        user: User,
        unreadMap: Map<string, Message[]>,
        currentUserId: number
    ): UiMenuItem {
        const key = MessageKeyConverter.toUnreadKey(
            currentUserId,
            user.idUser,
            MessageRecipientType.user
        );
        const count = unreadMap.get(key)?.length ?? 0;
        return {
            label: user.name,
            icon: 'user',
            badge: count > 0 ? count.toString() : undefined,
            badgeSeverity: 'danger',
            routerLink: ['/message', user.idUser, MessageRecipientType.user, 'view']
        };
    }
}

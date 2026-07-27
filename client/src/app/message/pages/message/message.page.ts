import {
    ChangeDetectionStrategy,
    Component,
    computed,
    effect,
    ElementRef,
    inject,
    OnDestroy,
    OnInit,
    signal,
    viewChild,
    viewChildren
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRoute } from '@angular/router';
import { TranslateService } from '@ngx-translate/core';
import { combineLatest, Observable, Subject, Subscription } from 'rxjs';
import { debounceTime, distinctUntilChanged, filter, map, switchMap, tap } from 'rxjs/operators';
import { User } from 'src/app/auth/model/user.model';
import { UserService } from 'src/app/auth/user.service';
import { Project } from 'src/app/project/model/project.model';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { ProjectService } from 'src/app/project/project.service';
import { Notice } from 'src/app/shared/notice/model/notice.model';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { Team } from 'src/app/team/model/team.model';
import { TeamMemberStore } from 'src/app/team/team-member.store';
import { TeamService } from 'src/app/team/team.service';
import { UserApi } from 'src/app/user/api/user.api.service';
import { MessageRecipientType } from '../../constant/message-recipient-type.enum';
import { ConversationGroup } from '../../entity/conversation-group.entity';
import { MessageService } from '../../message.service';
import { Message } from '../../model/message.model';

@Component({
    selector: 'app-message',
    templateUrl: './message.page.html',
    styleUrls: ['./message.page.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class MessagePage implements OnInit, OnDestroy {
    private readonly route = inject(ActivatedRoute);
    private readonly sMessage = inject(MessageService);
    private readonly sProject = inject(ProjectService);
    private readonly sTeam = inject(TeamService);
    private readonly userApi = inject(UserApi);
    private readonly sUser = inject(UserService);
    private readonly sNotice = inject(NoticeService);
    private readonly i18n = inject(TranslateService);
    private readonly projectMemberStore = inject(ProjectMemberStore);
    private readonly teamMemberStore = inject(TeamMemberStore);

    private readonly msgList = viewChild<ElementRef>('msgList');
    private readonly msgListItems = viewChildren('msgListItems');

    protected readonly conversationGroups = signal<ConversationGroup[]>([]);
    protected readonly messages = signal<Message[]>([]);
    protected readonly currentConversationName = signal<string>('');
    protected readonly idMessageEdit = signal<number | null>(null);
    protected readonly currentUserId = this.sUser.user.getValue().idUser;

    // Active conversation context — set in the paramMap tap alongside the imperative fields
    private readonly activeRecipientId = signal<number>(0);
    private readonly activeRecipientType = signal<number>(0);

    // All users loaded for DM peer resolution
    private readonly allUsers = signal<User[]>([]);

    // Member lists from stores, converted to signals
    private readonly projectMembers = toSignal(this.projectMemberStore.users$, {
        initialValue: [] as User[]
    });
    private readonly teamMembers = toSignal(this.teamMemberStore.users$, {
        initialValue: [] as User[]
    });

    public readonly mentionCandidates = computed<User[]>(() => {
        const type = this.activeRecipientType();
        const id = this.activeRecipientId();
        switch (type) {
            case MessageRecipientType.project:
                return this.projectMembers();
            case MessageRecipientType.team:
                return this.teamMembers();
            case MessageRecipientType.user: {
                const peer = this.allUsers().find(u => u.idUser === id);
                return peer ? [peer] : [];
            }
            default:
                return [];
        }
    });

    protected unreadCounts$: Map<string, Observable<number>> = new Map();

    private idRecipient: number;
    private idMessageRecipientType: number;
    private readonly setAsReadSignal = new Subject<{
        idRecipient: number;
        idMessageRecipientType: number;
    }>();
    private readonly subscriptions = new Subscription();

    private readonly scrollEffect = effect(() => {
        const items = this.msgListItems();
        if (items.length > 0) {
            this.scrollToBottom();
        }
    });

    public ngOnInit(): void {
        // messages
        this.subscriptions.add(
            this.route.paramMap
                .pipe(
                    tap(params => {
                        const recipientId = Number(params.get('idRecipient'));
                        const recipientType = Number(params.get('idMessageRecipientType'));
                        this.idRecipient = recipientId;
                        this.idMessageRecipientType = recipientType;
                        this.activeRecipientId.set(recipientId);
                        this.activeRecipientType.set(recipientType);
                        if (recipientType === MessageRecipientType.project) {
                            this.projectMemberStore.load(recipientId);
                        } else if (recipientType === MessageRecipientType.team) {
                            this.teamMemberStore.load(recipientId);
                        }
                    }),
                    switchMap(params =>
                        this.sMessage.loadMessages(
                            Number(params.get('idRecipient')),
                            Number(params.get('idMessageRecipientType'))
                        )
                    )
                )
                .subscribe(messages => {
                    this.messages.set(messages.reverse());
                    this.setAsReadSignal.next({
                        idRecipient: this.idRecipient,
                        idMessageRecipientType: this.idMessageRecipientType
                    });
                    this.findCurrentConversationName();
                })
        );

        // recipients
        this.subscriptions.add(
            combineLatest([
                this.sProject.loadProjects(),
                this.sTeam.loadMyTeams(),
                this.userApi.loadUsers$()
            ])
                .pipe(
                    map(([projects, teams, users]) => ({
                        conversationGroups: this.toConversationGroups(projects, teams, users),
                        users
                    }))
                )
                .subscribe(({ conversationGroups, users }) => {
                    this.conversationGroups.set(conversationGroups);
                    this.allUsers.set(users);
                    this.buildUnreadCounts(conversationGroups);
                    this.findCurrentConversationName();
                })
        );

        // live
        this.subscriptions.add(
            this.sNotice.Message.pipe(filter(notice => this.filterLiveMessage(notice))).subscribe(
                notice => {
                    if (notice.action === NoticeAction.Update) {
                        this.messages.update(msgs =>
                            msgs.map(m =>
                                m.idMessage === notice.payload.idMessage
                                    ? {
                                          ...m,
                                          message: notice.payload.message,
                                          updatedAt: notice.payload.updatedAt
                                      }
                                    : m
                            )
                        );
                        return;
                    }
                    this.messages.update(msgs =>
                        msgs.some(m => m.idMessage === notice.payload.idMessage)
                            ? msgs
                            : [...msgs, notice.payload]
                    );
                    this.setAsReadSignal.next({
                        idRecipient: this.idRecipient,
                        idMessageRecipientType: this.idMessageRecipientType
                    });
                }
            )
        );

        // set read
        this.subscriptions.add(
            this.setAsReadSignal
                .pipe(
                    debounceTime(1000),
                    switchMap(({ idRecipient, idMessageRecipientType }) =>
                        this.sMessage
                            .insertReadMessage(idRecipient, idMessageRecipientType)
                            .pipe(map(() => ({ idRecipient, idMessageRecipientType })))
                    )
                )
                .subscribe(({ idRecipient, idMessageRecipientType }) => {
                    const recipientKey =
                        idMessageRecipientType === MessageRecipientType.user
                            ? this.sUser.user.getValue().idUser
                            : idRecipient;
                    const creatorKey =
                        idMessageRecipientType === MessageRecipientType.user ? idRecipient : null;
                    this.sMessage.unreadRemove(recipientKey, creatorKey, idMessageRecipientType);
                    this.messages.update(msgs => msgs.map(m => ({ ...m, isRead: true })));
                })
        );
    }

    public ngOnDestroy(): void {
        this.subscriptions.unsubscribe();
    }

    protected getUnreadCount$(key: string): Observable<number> {
        return this.unreadCounts$.get(key);
    }

    protected formatCount(cnt: number): string {
        return cnt > 99 ? '99+' : String(cnt);
    }

    protected isNewDay(index: number): boolean {
        if (index === 0) return true;
        const prev = this.messages()[index - 1];
        const curr = this.messages()[index];
        return new Date(prev.createdAt).toDateString() !== new Date(curr.createdAt).toDateString();
    }

    protected isContinuationMsg(index: number): boolean {
        if (index === 0) return false;
        const prev = this.messages()[index - 1];
        const curr = this.messages()[index];
        const sameAuthor = prev.creator?.idUser === curr.creator?.idUser;
        const withinFiveMinutes =
            new Date(curr.createdAt).getTime() - new Date(prev.createdAt).getTime() < 5 * 60 * 1000;
        return sameAuthor && withinFiveMinutes;
    }

    protected onMessage(message: string): void {
        this.sMessage
            .insertMessage(this.idRecipient, this.idMessageRecipientType, message)
            .subscribe(savedMessage => {
                savedMessage.isRead = true;
                this.messages.update(msgs =>
                    msgs.some(m => m.idMessage === savedMessage.idMessage)
                        ? msgs
                        : [...msgs, savedMessage]
                );
            });
    }

    protected onEditRequest(message: Message): void {
        this.idMessageEdit.set(message.idMessage);
    }

    protected onEditCancel(): void {
        this.idMessageEdit.set(null);
    }

    protected onEditSave(message: Message, newText: string): void {
        this.sMessage.updateMessage(message.idMessage, newText).subscribe(updated => {
            this.messages.update(msgs =>
                msgs.map(m =>
                    m.idMessage === updated.idMessage
                        ? { ...m, message: updated.message, updatedAt: updated.updatedAt }
                        : m
                )
            );
            this.idMessageEdit.set(null);
        });
    }

    private findCurrentConversationName(): void {
        for (const group of this.conversationGroups()) {
            const found = group.conversations.find(
                c =>
                    c.idRecipient === this.idRecipient &&
                    c.idMessageRecipientType === this.idMessageRecipientType
            );
            if (found) {
                this.currentConversationName.set(found.name);
                return;
            }
        }
    }

    private buildUnreadCounts(groups: ConversationGroup[]): void {
        this.unreadCounts$.clear();
        for (const group of groups) {
            for (const c of group.conversations) {
                this.unreadCounts$.set(
                    c.unreadKey,
                    this.sMessage.Unread.pipe(
                        map(allUnread => allUnread.get(c.unreadKey)?.length),
                        distinctUntilChanged()
                    )
                );
            }
        }
    }

    private toConversationGroups(
        projects: Project[],
        teams: Team[],
        users: User[]
    ): ConversationGroup[] {
        const currentUserId = this.sUser.user.getValue().idUser;
        return [
            {
                name: this.i18n.instant('PROJECT.CHATS'),
                icon: 'messages',
                conversations: projects.map(p => ({
                    idRecipient: p.idProject,
                    idCreator: null,
                    idMessageRecipientType: MessageRecipientType.project,
                    name: p.name,
                    url: ['/message', p.idProject, MessageRecipientType.project, 'view'],
                    unreadKey: `${p.idProject}|null|${MessageRecipientType.project}`
                }))
            },
            {
                name: this.i18n.instant('TEAM.CHATS'),
                icon: 'users',
                conversations: teams.map(t => ({
                    idRecipient: t.idTeam,
                    idCreator: null,
                    idMessageRecipientType: MessageRecipientType.team,
                    name: t.name,
                    url: ['/message', t.idTeam, MessageRecipientType.team, 'view'],
                    unreadKey: `${t.idTeam}|null|${MessageRecipientType.team}`
                }))
            },
            {
                name: this.i18n.instant('DIRECT.CHATS'),
                icon: 'user',
                conversations: users
                    .filter(u => !u.isBot)
                    .map(u => ({
                        idRecipient: currentUserId,
                        idCreator: u.idUser,
                        idMessageRecipientType: MessageRecipientType.user,
                        name: u.name,
                        url: ['/message', u.idUser, MessageRecipientType.user, 'view'],
                        unreadKey: `${currentUserId}|${u.idUser}|${MessageRecipientType.user}`
                    }))
            }
        ];
    }

    private scrollToBottom(): void {
        setTimeout(() => {
            const el = this.msgList()?.nativeElement;
            if (el) el.scrollTop = el.scrollHeight;
        }, 0);
    }

    private filterLiveMessage(notice: Notice<Message>): boolean {
        if (this.idMessageRecipientType === MessageRecipientType.user) {
            return notice.payload.creator.idUser === this.idRecipient;
        }
        return (
            notice.payload.idRecipient === this.idRecipient &&
            notice.payload.idMessageRecipientType === this.idMessageRecipientType
        );
    }
}

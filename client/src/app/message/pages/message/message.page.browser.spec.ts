import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { Component, input, output, signal } from '@angular/core';
import { By } from '@angular/platform-browser';
import { ActivatedRoute } from '@angular/router';
import { RouterTestingModule } from '@angular/router/testing';
import { BehaviorSubject, NEVER, of } from 'rxjs';
import { filter } from 'rxjs/operators';
import { TranslateModule } from '@ngx-translate/core';

import { MessagePage } from './message.page';
import { MessageRecipientType } from '../../constant/message-recipient-type.enum';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { TeamMemberStore } from 'src/app/team/team-member.store';
import { MessageService } from '../../message.service';
import { ProjectService } from 'src/app/project/project.service';
import { TeamService } from 'src/app/team/team.service';
import { UserApi } from 'src/app/user/api/user.api.service';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { MessageEditorStub, TablerIconStub, UiBadgeStub } from 'src/testing/stubs';
import { User } from 'src/app/auth/model/user.model';
import { Project } from 'src/app/project/model/project.model';
import { Message } from '../../model/message.model';
import { MessageKeyConverter } from '../../converter/message-key.converter';
import { MessageKind } from '../../constant/message-kind.enum';

function unreadMessage(idMessage: number): Message {
    return {
        idMessage,
        message: 'hi',
        messageKind: MessageKind.Comment,
        createdAt: new Date('2026-08-01T00:00:00Z'),
        isRead: false,
        creator: { idUser: 1, name: 'Ada', email: 'ada@x.io', colorAvatarBg: '#123456' },
        idRecipient: 7,
        idMessageRecipientType: MessageRecipientType.project,
        version: 1,
        anchor: null
    };
}

@Component({ selector: 'app-message-view', template: '', standalone: true })
class MessageViewStub {
    public readonly message = input<unknown>(undefined);
    public readonly isContinuation = input<boolean>(false);
    public readonly isOwnMessage = input<boolean>(false);
    public readonly isEditing = input<boolean>(false);
    public readonly candidates = input<unknown>(undefined);
    public readonly mentionCandidates = input<User[]>([]);
    public readonly editRequest = output<void>();
    public readonly editSave = output<string>();
    public readonly editCancel = output<void>();
}

const alice: User = { idUser: 1, name: 'Alice', email: 'alice@test.com', colorAvatarBg: '#aaa' };
const bob: User = { idUser: 2, name: 'Bob', email: 'bob@test.com', colorAvatarBg: '#bbb' };
const carol: User = { idUser: 3, name: 'Carol', email: 'carol@test.com', colorAvatarBg: '#ccc' };

// Minimal param map factory
function makeParamMap(idRecipient: number, idMessageRecipientType: number) {
    return {
        get: (key: string) => {
            if (key === 'idRecipient') return String(idRecipient);
            if (key === 'idMessageRecipientType') return String(idMessageRecipientType);
            return null;
        }
    };
}

describe('MessagePage mentionCandidates', () => {
    let teamUsersSubject: BehaviorSubject<User[] | null>;
    let projectUsersSubject: BehaviorSubject<User[] | null>;
    let paramMapSubject: BehaviorSubject<ReturnType<typeof makeParamMap>>;
    let unreadSubject: BehaviorSubject<Map<string, Message[]>>;
    let projects: Project[];

    beforeEach(async () => {
        teamUsersSubject = new BehaviorSubject<User[] | null>(null);
        projectUsersSubject = new BehaviorSubject<User[] | null>(null);
        paramMapSubject = new BehaviorSubject(makeParamMap(0, 0));
        unreadSubject = new BehaviorSubject<Map<string, Message[]>>(new Map());
        projects = [];

        const teamMemberStoreStub: Partial<TeamMemberStore> = {
            users$: teamUsersSubject.asObservable().pipe(filter((v): v is User[] => v !== null)),
            load: () => {
                /* no-op in tests */
            }
        };

        const projectMemberStoreStub: Partial<ProjectMemberStore> = {
            users$: projectUsersSubject.asObservable().pipe(filter((v): v is User[] => v !== null)),
            load: () => {
                /* no-op in tests */
            }
        };

        await TestBed.configureTestingModule({
            declarations: [MessagePage],
            imports: [
                TranslateModule.forRoot(),
                RouterTestingModule,
                MessageViewStub,
                MessageEditorStub,
                TablerIconStub,
                UiBadgeStub
            ],
            providers: [
                {
                    provide: ActivatedRoute,
                    useValue: { paramMap: paramMapSubject.asObservable() }
                },
                {
                    provide: MessageService,
                    useValue: {
                        loadMessages: () => of([]),
                        Unread: unreadSubject,
                        insertReadMessage: () => of(null),
                        unreadRemove: () => {}
                    }
                },
                {
                    provide: ProjectService,
                    useValue: { loadProjects: () => of(projects), loadMembers: () => of([]) }
                },
                {
                    provide: TeamService,
                    useValue: { loadMyTeams: () => of([]) }
                },
                {
                    provide: UserApi,
                    useValue: { loadUsers$: () => of([alice, bob, carol]) }
                },
                {
                    provide: AuthStore,
                    useValue: {
                        user: signal<User>({
                            idUser: 99,
                            name: 'Me',
                            email: '',
                            colorAvatarBg: ''
                        }),
                        getUser: () => ({
                            idUser: 99,
                            name: 'Me',
                            email: '',
                            colorAvatarBg: ''
                        })
                    }
                },
                {
                    provide: NoticeService,
                    useValue: { Message: NEVER }
                },
                { provide: TeamMemberStore, useValue: teamMemberStoreStub },
                { provide: ProjectMemberStore, useValue: projectMemberStoreStub }
            ]
        }).compileComponents();
    });

    function createPage(): MessagePage {
        const fixture = TestBed.createComponent(MessagePage);
        fixture.detectChanges();
        return fixture.componentInstance;
    }

    it('returns team members when conversation type is team', () => {
        // Navigate to a team conversation
        paramMapSubject.next(makeParamMap(42, MessageRecipientType.team));
        // Emit team members from the store
        teamUsersSubject.next([alice, bob]);

        const page = createPage();
        // detectChanges already called in createPage; paramMap already emitted
        expect(page.mentionCandidates()).toEqual([alice, bob]);
    });

    it('returns project members when conversation type is project', () => {
        paramMapSubject.next(makeParamMap(10, MessageRecipientType.project));
        projectUsersSubject.next([carol]);

        const page = createPage();
        expect(page.mentionCandidates()).toEqual([carol]);
    });

    it('returns [peer] when conversation type is user (DM)', () => {
        // DM to bob (idUser=2)
        paramMapSubject.next(makeParamMap(2, MessageRecipientType.user));

        const page = createPage();
        // allUsers is populated from userApi.loadUsers$() which returns [alice, bob, carol]
        expect(page.mentionCandidates()).toEqual([bob]);
    });

    it('returns [] when switching from team to unknown type', () => {
        paramMapSubject.next(makeParamMap(99, MessageRecipientType.issue));

        const page = createPage();
        expect(page.mentionCandidates()).toEqual([]);
    });

    it('updates candidates reactively when conversation switches from team to DM', () => {
        // Start on team
        paramMapSubject.next(makeParamMap(42, MessageRecipientType.team));
        teamUsersSubject.next([alice, bob]);

        const page = createPage();
        expect(page.mentionCandidates()).toEqual([alice, bob]);

        // Switch to DM with alice
        paramMapSubject.next(makeParamMap(1, MessageRecipientType.user));
        expect(page.mentionCandidates()).toEqual([alice]);
    });

    it('shows no unread badge for a conversation with nothing unread', () => {
        projects = [{ idProject: 7, name: 'Proj', color: '' }];
        paramMapSubject.next(makeParamMap(7, MessageRecipientType.project));

        const fixture = TestBed.createComponent(MessagePage);
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelectorAll('ui-badge').length).toBe(0);
    });

    it('renders the unread count of a conversation and follows later Unread emissions', () => {
        projects = [{ idProject: 7, name: 'Proj', color: '' }];
        paramMapSubject.next(makeParamMap(7, MessageRecipientType.project));

        const fixture = TestBed.createComponent(MessagePage);
        fixture.detectChanges();

        const key = MessageKeyConverter.toUnreadKey(7, null, MessageRecipientType.project);
        unreadSubject.next(new Map([[key, [unreadMessage(1), unreadMessage(2)]]]));
        fixture.detectChanges();

        const badge = fixture.debugElement.query(By.directive(UiBadgeStub));
        expect(badge.componentInstance.value()).toBe('2');
    });
});

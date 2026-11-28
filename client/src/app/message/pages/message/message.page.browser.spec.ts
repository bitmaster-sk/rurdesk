import { describe, it, expect, beforeEach, onTestFinished } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { Component, input, output, signal } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { RouterTestingModule } from '@angular/router/testing';
import { BehaviorSubject, NEVER, Observable, Subject, of, throwError } from 'rxjs';
import { filter } from 'rxjs/operators';
import { TranslateModule, TranslateService } from '@ngx-translate/core';

import { MessagePage } from './message.page';
import { MessageRecipientType } from '../../constant/message-recipient-type.enum';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { TeamMemberStore } from 'src/app/team/team-member.store';
import { MessageApi } from '../../api/message.api.service';
import { MessageUnreadStore } from '../../store/message-unread.store';
import { ProjectApi } from 'src/app/project/api/project.api.service';
import { TeamApi } from 'src/app/team/api/team.api.service';
import { UserApi } from 'src/app/user/api/user.api.service';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { LinkScope } from 'src/app/link/service/link-scope.service';
import { IconMessages, IconUser, IconUsers, provideTablerIcons } from '@tabler/icons-angular';
import { MessageEditorStub, TablerIconStub } from 'src/testing/stubs';
import { UiModule } from 'src/app/ui/ui.module';
import { User } from 'src/app/auth/model/user.model';
import { Project } from 'src/app/project/model/project.model';
import { Team } from 'src/app/team/model/team.model';
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
    let loadMessages: (idRecipient: number, idType: number) => Observable<Message[]>;
    let teams$: Observable<Team[]>;

    beforeEach(async () => {
        loadMessages = () => of([]);
        teams$ = of([]);
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
                UiModule
            ],
            providers: [
                provideTablerIcons({ IconMessages, IconUsers, IconUser }),
                {
                    provide: ActivatedRoute,
                    useValue: { paramMap: paramMapSubject.asObservable() }
                },
                {
                    provide: MessageApi,
                    useValue: {
                        load$: (idRecipient: number, idType: number) =>
                            loadMessages(idRecipient, idType),
                        markRead$: () => of(null)
                    }
                },
                {
                    provide: MessageUnreadStore,
                    useValue: { unread$: unreadSubject, remove: () => {} }
                },
                {
                    provide: ProjectApi,
                    useValue: { load$: () => of(projects), loadMembers$: () => of([]) }
                },
                {
                    provide: TeamApi,
                    useValue: { loadMy$: () => teams$ }
                },
                {
                    provide: UserApi,
                    useValue: { load$: () => of([alice, bob, carol]) }
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
        // allUsers is populated from userApi.load$() which returns [alice, bob, carol]
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

        expect(
            fixture.nativeElement.querySelectorAll('[data-testid="message-unread"]').length
        ).toBe(0);
    });

    it('renders the unread count of a conversation and follows later Unread emissions', () => {
        projects = [{ idProject: 7, name: 'Proj', color: '' }];
        paramMapSubject.next(makeParamMap(7, MessageRecipientType.project));

        const fixture = TestBed.createComponent(MessagePage);
        fixture.detectChanges();

        const key = MessageKeyConverter.toUnreadKey(7, null, MessageRecipientType.project);
        unreadSubject.next(new Map([[key, [unreadMessage(1), unreadMessage(2)]]]));
        fixture.detectChanges();

        const count = fixture.nativeElement.querySelector('[data-testid="message-unread"]');
        expect(count.textContent.trim()).toBe('2');
    });

    it('renders conversation group headings via translate pipe and reacts to language changes', () => {
        const translate = TestBed.inject(TranslateService);

        projects = [{ idProject: 7, name: 'Proj', color: '' }];
        paramMapSubject.next(makeParamMap(7, MessageRecipientType.project));

        const fixture = TestBed.createComponent(MessagePage);
        fixture.detectChanges();

        const headings = fixture.nativeElement.querySelectorAll('.ui-side-panel-section-title');
        expect(headings.length).toBe(3);
        expect(headings[0].textContent.trim()).toBe(translate.instant('PROJECT.CHATS'));
        expect(headings[1].textContent.trim()).toBe(translate.instant('TEAM.CHATS'));
        expect(headings[2].textContent.trim()).toBe(translate.instant('DIRECT.CHATS'));

        // Simulate a language switch without reloading the page
        translate.use('en');
        translate.setTranslation('sk', {
            PROJECT: { CHATS: 'Projektové chaty' },
            TEAM: { CHATS: 'Tímové chaty' },
            DIRECT: { CHATS: 'Priame správy' }
        });
        translate.use('sk');
        fixture.detectChanges();

        const newHeadings = fixture.nativeElement.querySelectorAll('.ui-side-panel-section-title');
        expect(newHeadings[0].textContent.trim()).toBe('Projektové chaty');
        expect(newHeadings[1].textContent.trim()).toBe('Tímové chaty');
        expect(newHeadings[2].textContent.trim()).toBe('Priame správy');
    });

    it('provides the project id to the link scope in a project chat', () => {
        paramMapSubject.next(makeParamMap(7, MessageRecipientType.project));

        const fixture = TestBed.createComponent(MessagePage);
        fixture.detectChanges();

        expect(fixture.debugElement.injector.get(LinkScope).idProject()).toBe(7);
    });

    it('clears the link scope in team and DM chats and refills it when switching back', () => {
        paramMapSubject.next(makeParamMap(7, MessageRecipientType.project));
        const fixture = TestBed.createComponent(MessagePage);
        fixture.detectChanges();
        const linkScope = fixture.debugElement.injector.get(LinkScope);
        expect(linkScope.idProject()).toBe(7);

        paramMapSubject.next(makeParamMap(42, MessageRecipientType.team));
        expect(linkScope.idProject()).toBeNull();

        paramMapSubject.next(makeParamMap(1, MessageRecipientType.user));
        expect(linkScope.idProject()).toBeNull();

        paramMapSubject.next(makeParamMap(9, MessageRecipientType.project));
        expect(linkScope.idProject()).toBe(9);
    });

    function renderChats(): {
        element: HTMLElement;
        detect: () => void;
        filter: (text: string) => void;
    } {
        projects = [
            { idProject: 7, name: 'Apollo', color: '' },
            { idProject: 8, name: 'Gemini', color: '' }
        ];
        paramMapSubject.next(makeParamMap(8, MessageRecipientType.project));
        const fixture = TestBed.createComponent(MessagePage);
        fixture.detectChanges();
        const element = fixture.nativeElement as HTMLElement;
        const detect = (): void => fixture.detectChanges();
        return {
            element,
            detect,
            filter: (text: string) => {
                const input = element.querySelector<HTMLInputElement>(
                    '[data-testid="message-filter"]'
                )!;
                input.value = text;
                input.dispatchEvent(new Event('input'));
                detect();
            }
        };
    }

    function rowNames(element: HTMLElement): string[] {
        return Array.from(
            element.querySelectorAll<HTMLElement>('[data-testid="message-conversation"]')
        ).map(row => row.title);
    }

    it('filters chats by name and drops empty groups', () => {
        const { element, filter } = renderChats();
        expect(rowNames(element)).toEqual(['Apollo', 'Gemini', 'Alice', 'Bob', 'Carol']);

        filter('BO');

        expect(rowNames(element)).toEqual(['Bob']);
        expect(element.querySelectorAll('ui-side-panel-section')).toHaveLength(1);
        expect(element.querySelector('.ui-side-panel-section-title')?.textContent?.trim()).toBe(
            TestBed.inject(TranslateService).instant('DIRECT.CHATS')
        );
    });

    it('says so when no chat matches', () => {
        const { element, filter } = renderChats();
        expect(element.querySelector('[data-testid="message-filter-empty"]')).toBeNull();

        filter('zzz');

        expect(rowNames(element)).toEqual([]);
        expect(element.querySelector('[data-testid="message-filter-empty"]')).not.toBeNull();
    });

    it('shows a loader instead of "no chat matches" until the chats arrive', () => {
        const teams = new Subject<Team[]>();
        teams$ = teams;
        const { element, detect } = renderChats();
        expect(element.querySelector('[data-testid="side-panel-loading"]')).not.toBeNull();
        expect(element.querySelector('[data-testid="message-filter-empty"]')).toBeNull();

        teams.next([]);
        detect();

        expect(element.querySelector('[data-testid="side-panel-loading"]')).toBeNull();
        expect(rowNames(element)).toEqual(['Apollo', 'Gemini', 'Alice', 'Bob', 'Carol']);
    });

    it('keeps opening chats after one the user can no longer read', () => {
        const message = unreadMessage(1);
        loadMessages = idRecipient =>
            idRecipient === 5 ? throwError(() => new Error('403')) : of([message]);
        paramMapSubject.next(makeParamMap(5, MessageRecipientType.team));
        const fixture = TestBed.createComponent(MessagePage);
        fixture.detectChanges();
        const element = fixture.nativeElement as HTMLElement;
        expect(element.querySelectorAll('app-message-view')).toHaveLength(0);

        paramMapSubject.next(makeParamMap(8, MessageRecipientType.project));
        fixture.detectChanges();

        expect(element.querySelectorAll('app-message-view')).toHaveLength(1);
    });

    it('remembers for the next visit only a chat whose messages loaded', () => {
        const key = 'message.lastConversation';
        onTestFinished(() => localStorage.removeItem(key));
        localStorage.setItem(key, '/message/5/2/view');
        loadMessages = idRecipient =>
            idRecipient === 5 ? throwError(() => new Error('403')) : of([]);

        paramMapSubject.next(makeParamMap(5, MessageRecipientType.team));
        TestBed.createComponent(MessagePage).detectChanges();
        expect(localStorage.getItem(key)).toBeNull();

        paramMapSubject.next(makeParamMap(8, MessageRecipientType.project));
        expect(localStorage.getItem(key)).toBe('/message/8/3/view');
    });

    it('marks the open chat as the current page', () => {
        const { element } = renderChats();

        const current = element.querySelectorAll<HTMLElement>('[aria-current="page"]');
        expect(Array.from(current).map(row => row.title)).toEqual(['Gemini']);
    });
});

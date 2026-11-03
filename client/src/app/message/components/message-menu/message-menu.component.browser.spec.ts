import { TestBed } from '@angular/core/testing';
import { BehaviorSubject, Subject, of } from 'rxjs';
import { describe, expect, it } from 'vitest';
import { User } from 'src/app/auth/model/user.model';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { MessageKind } from 'src/app/message/constant/message-kind.enum';
import { MessageRecipientType } from 'src/app/message/constant/message-recipient-type.enum';
import { MessageKeyConverter } from 'src/app/message/converter/message-key.converter';
import { Message } from 'src/app/message/model/message.model';
import { MessageUnreadStore } from 'src/app/message/store/message-unread.store';
import { ProjectApi } from 'src/app/project/api/project.api.service';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { TeamApi } from 'src/app/team/api/team.api.service';
import { UiMenuItem } from 'src/app/ui/components/menu/menu-item.model';
import { UserApi } from 'src/app/user/api/user.api.service';
import { MessageMenuComponent } from './message-menu.component';

const ME = 1;
const active: User = { idUser: 2, name: 'Ada', email: '', colorAvatarBg: '#000' };
const gone: User = { idUser: 3, name: 'Gone', email: '', colorAvatarBg: '#000', isActive: false };
const dmFromGone: Message = {
    idMessage: 10,
    message: 'hi',
    messageKind: MessageKind.Comment,
    isRead: false,
    createdAt: new Date('2026-01-01T10:00:00Z'),
    idRecipient: ME,
    idMessageRecipientType: MessageRecipientType.user,
    creator: gone,
    version: 1,
    anchor: null
};

async function directChatLabels(unread: Map<string, Message[]>): Promise<string[]> {
    await TestBed.configureTestingModule({
        declarations: [MessageMenuComponent],
        providers: [
            {
                provide: MessageUnreadStore,
                useValue: { unread$: new BehaviorSubject(unread), load: () => {}, push: () => {} }
            },
            { provide: AuthStore, useValue: { getUser: () => ({ idUser: ME }) } },
            { provide: ProjectApi, useValue: { load$: () => of([]) } },
            { provide: TeamApi, useValue: { loadMy$: () => of([]) } },
            { provide: UserApi, useValue: { load$: () => of([active, gone]) } },
            { provide: NoticeService, useValue: { Message: new Subject() } }
        ]
    })
        .overrideComponent(MessageMenuComponent, { set: { template: '' } })
        .compileComponents();

    const fixture = TestBed.createComponent(MessageMenuComponent);
    fixture.detectChanges();
    const chats = (fixture.componentInstance as unknown as { chats(): UiMenuItem[] }).chats();
    const direct = chats.find(group => group.labelKey === 'DIRECT.CHATS');
    return (direct?.items ?? []).map(item => item.label ?? '');
}

describe('MessageMenuComponent — disabled users (browser)', () => {
    it('leaves out a disabled user without unread messages', async () => {
        expect(await directChatLabels(new Map())).toEqual(['Ada']);
    });

    it('keeps a disabled user with unread messages so the count can be cleared', async () => {
        const key = MessageKeyConverter.toUnreadKey(ME, gone.idUser, MessageRecipientType.user);
        const unread = new Map<string, Message[]>([[key, [dmFromGone]]]);

        expect(await directChatLabels(unread)).toEqual(['Ada', 'Gone']);
    });
});

import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { TablerIconComponent, provideTablerIcons } from '@tabler/icons-angular';
import { AvatarStub, MessageBodyStub, MessageEditorStub } from 'src/testing/stubs';
import { AnchorThreadComponent } from './anchor-thread.component';
import { ISSUE_ICONS } from 'src/app/issue/constants/issue-icons.constant';
import { AnchorThread } from 'src/app/issue/entity/anchor-thread.entity';
import { MessageKind } from 'src/app/message/constant/message-kind.enum';
import { Message } from 'src/app/message/model/message.model';
import { User } from 'src/app/auth/model/user.model';

const JAN: User = { idUser: 3, name: 'Jan', email: '', colorAvatarBg: '' };

function thread(): AnchorThread {
    return {
        idThread: 5,
        idParentMessage: 1,
        lineStart: 1,
        lineEnd: 1,
        anchorText: '',
        isOutdated: false,
        replies: [
            {
                idMessage: 5,
                message: 'reply text',
                messageKind: MessageKind.Comment,
                isRead: true,
                createdAt: new Date(),
                creator: JAN
            } as unknown as Message
        ]
    };
}

/** The other icons the anchored-reply views name: the selection reply and the way back. */
@Component({
    selector: 'app-icon-probe',
    template: `
        <tabler-icon icon="message-reply" />
        <tabler-icon icon="arrow-back-up" />
    `,
    standalone: true,
    imports: [TablerIconComponent]
})
class IconProbe {}

describe('AnchorThreadComponent with the issue icons (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                TranslateModule.forRoot(),
                TablerIconComponent,
                AvatarStub,
                MessageBodyStub,
                MessageEditorStub,
                IconProbe
            ],
            declarations: [AnchorThreadComponent],
            providers: [provideTablerIcons(ISSUE_ICONS)]
        }).compileComponents();
    });

    it('shows the replies of a thread opened by a click, without a further render', async () => {
        const fixture = TestBed.createComponent(AnchorThreadComponent);
        fixture.componentRef.setInput('thread', thread());
        fixture.autoDetectChanges();
        await fixture.whenStable();
        const el = fixture.nativeElement as HTMLElement;
        el.querySelector<HTMLElement>('[data-testid="anchor-thread-toggle"]')!.addEventListener(
            'click',
            () => fixture.componentRef.setInput('isExpanded', true)
        );

        el.querySelector<HTMLElement>('[data-testid="anchor-thread-toggle"]')!.click();
        await fixture.whenStable();

        expect(el.querySelector('[data-testid="anchor-thread-reply"]')?.textContent).toContain(
            'Jan'
        );
        expect(el.querySelector('.chevron svg')).not.toBeNull();
    });

    it('draws the reply and back icons', async () => {
        const fixture = TestBed.createComponent(IconProbe);
        fixture.detectChanges();
        await fixture.whenStable();

        expect((fixture.nativeElement as HTMLElement).querySelectorAll('svg')).toHaveLength(2);
    });
});

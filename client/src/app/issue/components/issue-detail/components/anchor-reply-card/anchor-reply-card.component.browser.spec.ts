import { Component, Pipe, PipeTransform, input } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { AvatarStub, UiTagStub } from 'src/testing/stubs';
import { AnchorReplyCardComponent } from './anchor-reply-card.component';
import { MessageKind } from 'src/app/message/constant/message-kind.enum';
import { User } from 'src/app/auth/model/user.model';

@Pipe({ name: 'date', standalone: false })
class StubDatePipe implements PipeTransform {
    public transform(value: unknown): string {
        return String(value);
    }
}

/** A body with what a rendered reply can hold: a link and an attachment thumbnail. */
@Component({
    selector: 'app-message-body',
    template: `
        <a href="#reply-link" (click)="$event.preventDefault()">link</a>
        <button type="button">thumbnail</button>
    `,
    standalone: true
})
class LinkedBodyStub {
    public readonly body = input<string>('');
    public readonly candidates = input<unknown>(undefined);
}

const JAN: User = { idUser: 3, name: 'Jan', email: '', colorAvatarBg: '' };

describe('AnchorReplyCardComponent (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [TranslateModule.forRoot(), AvatarStub, UiTagStub, LinkedBodyStub],
            declarations: [AnchorReplyCardComponent, StubDatePipe]
        }).compileComponents();
    });

    function render(): { el: HTMLElement; opened: () => number } {
        const fixture = TestBed.createComponent(AnchorReplyCardComponent);
        fixture.componentRef.setInput('reply', {
            idMessage: 9,
            message: 'reply',
            messageKind: MessageKind.Comment,
            isRead: true,
            createdAt: new Date(),
            creator: JAN,
            anchor: {
                idParentMessage: 1,
                parentVersion: 1,
                anchorLineStart: 1,
                anchorLineEnd: 1,
                isOutdated: false,
                anchorText: 'text',
                quote: 'quoted'
            }
        });
        let count = 0;
        fixture.componentInstance.open.subscribe(() => count++);
        fixture.detectChanges();
        return { el: fixture.nativeElement as HTMLElement, opened: () => count };
    }

    it('opens the thread when the card is clicked', () => {
        const { el, opened } = render();

        el.querySelector<HTMLElement>('.quote')!.click();

        expect(opened()).toBe(1);
    });

    it('leaves the thread closed when a link or attachment in the reply is clicked', () => {
        const { el, opened } = render();

        el.querySelector<HTMLElement>('.body a')!.click();
        el.querySelector<HTMLElement>('.body button')!.click();

        expect(opened()).toBe(0);
    });
});

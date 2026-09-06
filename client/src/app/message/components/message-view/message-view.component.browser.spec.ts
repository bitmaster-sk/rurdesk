import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ComponentRef } from '@angular/core';
import {
    AvatarStub,
    MessageBodyStub,
    MessageEditorStub,
    TablerIconStub,
    UiButtonStub
} from 'src/testing/stubs';
import { MessageViewComponent } from './message-view.component';
import { Message } from '../../model/message.model';
import { MessageRecipientType } from '../../constant/message-recipient-type.enum';
import { MessageKind } from '../../constant/message-kind.enum';
import { TranslateModule } from '@ngx-translate/core';

const mockMessage = (overrides: Partial<Message> = {}): Message => ({
    idMessage: 1,
    message: 'Hello world',
    messageKind: MessageKind.Comment,
    isRead: true,
    createdAt: new Date('2026-01-01T10:00:00Z'),
    idRecipient: 1,
    idMessageRecipientType: MessageRecipientType.project,
    creator: { idUser: 42, name: 'Alice', colorAvatarBg: '#000', email: '' },
    version: 1,
    anchor: null,
    ...overrides
});

describe('MessageViewComponent', () => {
    let component: MessageViewComponent;
    let fixture: ComponentFixture<MessageViewComponent>;
    let componentRef: ComponentRef<MessageViewComponent>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [MessageViewComponent],
            imports: [
                TranslateModule.forRoot(),
                AvatarStub,
                MessageBodyStub,
                MessageEditorStub,
                TablerIconStub,
                UiButtonStub
            ]
        }).compileComponents();

        fixture = TestBed.createComponent(MessageViewComponent);
        component = fixture.componentInstance;
        componentRef = fixture.componentRef;
        componentRef.setInput('message', mockMessage());
        fixture.detectChanges();
    });

    it('should create', () => {
        expect(component).toBeTruthy();
    });

    describe('edit button', () => {
        it('is not rendered when isOwnMessage is false', () => {
            componentRef.setInput('isOwnMessage', false);
            fixture.detectChanges();
            const btn = fixture.nativeElement.querySelector('.edit-btn');
            expect(btn).toBeNull();
        });

        it('is rendered when isOwnMessage is true', () => {
            componentRef.setInput('isOwnMessage', true);
            fixture.detectChanges();
            const btn = fixture.nativeElement.querySelector('.edit-btn');
            expect(btn).not.toBeNull();
        });

        it('emits editRequest when clicked', () => {
            componentRef.setInput('isOwnMessage', true);
            fixture.detectChanges();
            let emitted = false;
            component.editRequest.subscribe(() => (emitted = true));
            fixture.nativeElement.querySelector('.edit-btn').click();
            expect(emitted).toBe(true);
        });
    });

    describe('edit mode', () => {
        it('hides message text and shows editor when isEditing is true', () => {
            componentRef.setInput('isEditing', true);
            fixture.detectChanges();
            const text = fixture.nativeElement.querySelector('.message-text');
            const editor = fixture.nativeElement.querySelector('app-message-editor');
            expect(text).toBeNull();
            expect(editor).not.toBeNull();
        });

        it('shows message text when isEditing is false', () => {
            componentRef.setInput('isEditing', false);
            fixture.detectChanges();
            const text = fixture.nativeElement.querySelector('.message-text');
            expect(text).not.toBeNull();
        });

        it('emits editCancel on Escape keydown', () => {
            componentRef.setInput('isEditing', true);
            fixture.detectChanges();
            let emitted = false;
            component.editCancel.subscribe(() => (emitted = true));
            const wrapper = fixture.nativeElement.querySelector('.edit-wrapper');
            wrapper.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
            expect(emitted).toBe(true);
        });
    });

    describe('edited label', () => {
        it('is not shown when updatedAt is not set', () => {
            componentRef.setInput('message', mockMessage({ updatedAt: undefined }));
            fixture.detectChanges();
            const label = fixture.nativeElement.querySelector('.message-edited');
            expect(label).toBeNull();
        });

        it('is shown when updatedAt is set', () => {
            componentRef.setInput(
                'message',
                mockMessage({ updatedAt: new Date('2026-01-01T11:00:00Z') })
            );
            fixture.detectChanges();
            const label = fixture.nativeElement.querySelector('.message-edited');
            expect(label).not.toBeNull();
        });
    });
});

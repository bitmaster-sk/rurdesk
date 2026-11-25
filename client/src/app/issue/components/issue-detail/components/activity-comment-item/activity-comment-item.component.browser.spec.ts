import { Component, input } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { MarkdownModule } from 'ngx-markdown';
import { MARKDOWN_MARKED_OPTIONS } from 'src/app/shared/markdown/marked-options';
import { MARKDOWN_MENTION_EXTENSION } from 'src/app/shared/markdown/mention-extension';
import { TranslateModule } from '@ngx-translate/core';
import { UiModule } from 'src/app/ui/ui.module';
import {
    AttachmentLightboxStub,
    AvatarStub,
    MessageEditorStub,
    TablerIconStub
} from 'src/testing/stubs';
import { ActivityCommentItemComponent } from './activity-comment-item.component';
import { MessageBodyComponent } from 'src/app/shared/mention/message-body/message-body.component';
import { MockupCardComponent } from 'src/app/shared/components/mockup-card/mockup-card.component';
import { MessageKind } from 'src/app/message/constant/message-kind.enum';
import { Message } from 'src/app/message/model/message.model';
import { AgentRun } from 'src/app/agent/model/agent-run.model';
import { AgentPhase } from 'src/app/agent/model/agent-phase.enum';

function awaitingApprovalRun(approvedMockupRef: string | null = null): AgentRun {
    return {
        idRun: 1,
        phase: AgentPhase.AwaitingApproval,
        approvedMockupRef
    } as unknown as AgentRun;
}

@Component({ selector: 'app-diff-viewer', template: '', standalone: true })
class DiffViewerStub {
    public readonly rawPatch = input<string>('');
}

function designMessage(body: string): Message {
    return {
        message: body,
        messageKind: MessageKind.Design,
        isRead: true,
        createdAt: new Date(),
        creator: { name: 'Agent' },
        anchor: null
    } as unknown as Message;
}

function plainMessage(body: string): Message {
    return {
        message: body,
        messageKind: MessageKind.Comment,
        isRead: true,
        createdAt: new Date(),
        creator: { name: 'User' },
        anchor: null
    } as unknown as Message;
}

describe('ActivityCommentItemComponent mockup rendering (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                MarkdownModule.forRoot({
                    markedOptions: MARKDOWN_MARKED_OPTIONS,
                    markedExtensions: [MARKDOWN_MENTION_EXTENSION]
                }),
                UiModule,
                TranslateModule.forRoot(),
                AvatarStub,
                MessageEditorStub,
                TablerIconStub,
                AttachmentLightboxStub,
                DiffViewerStub
            ],
            declarations: [ActivityCommentItemComponent, MessageBodyComponent, MockupCardComponent]
        }).compileComponents();
    });

    function render(body: string) {
        const fixture = TestBed.createComponent(ActivityCommentItemComponent);
        fixture.componentRef.setInput('message', designMessage(body));
        fixture.detectChanges();
        return fixture;
    }

    it('renders a mockup card for a design message containing a ```mockup block', () => {
        const fixture = render('Here is the design:\n```mockup\n<h1>Hi</h1>\n```');

        const cards = fixture.nativeElement.querySelectorAll('app-mockup-card');
        expect(cards.length).toBe(1);
    });

    it('renders no mockup card for a design message without one', () => {
        const fixture = render('Just a plain design write-up, no mockup.');

        const cards = fixture.nativeElement.querySelectorAll('app-mockup-card');
        expect(cards.length).toBe(0);
    });

    it('marks a comment whose author is disabled', () => {
        const fixture = TestBed.createComponent(ActivityCommentItemComponent);
        const message = plainMessage('hello');
        message.creator = { ...message.creator, isActive: false };
        fixture.componentRef.setInput('message', message);
        fixture.detectChanges();

        expect(
            fixture.nativeElement.querySelector('[data-testid="author-disabled"]')
        ).not.toBeNull();
        expect(fixture.nativeElement.querySelector('.author--disabled')).not.toBeNull();
    });

    it('does not mark a comment whose author is active', () => {
        const fixture = TestBed.createComponent(ActivityCommentItemComponent);
        fixture.componentRef.setInput('message', plainMessage('hello'));
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelector('[data-testid="author-disabled"]')).toBeNull();
    });

    it('re-emits approveMockup from the card of a design message with mockups', () => {
        const fixture = TestBed.createComponent(ActivityCommentItemComponent);
        fixture.componentRef.setInput(
            'message',
            designMessage('```mockup title="B"\n<p>b</p>\n```')
        );
        fixture.componentRef.setInput('agentRun', awaitingApprovalRun());
        fixture.componentRef.setInput('isLatestPlanForStage', true);
        let ref = '';
        fixture.componentInstance.approveMockup.subscribe(r => (ref = r));
        fixture.detectChanges();

        const btn = fixture.nativeElement.querySelector('.mockup-card__approve') as HTMLElement;
        expect(btn).toBeTruthy();
        btn.click();
        expect(ref).toBe('B #1');
    });

    it('offers no Approve of its own for a design without mockups: the feedback bar has it', () => {
        const fixture = TestBed.createComponent(ActivityCommentItemComponent);
        fixture.componentRef.setInput('message', designMessage('Plain design, no mockup.'));
        fixture.componentRef.setInput('agentRun', awaitingApprovalRun());
        fixture.componentRef.setInput('isLatestPlanForStage', true);
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelector('ui-button')).toBeNull();
        expect(fixture.nativeElement.querySelector('.mockup-card__approve')).toBeNull();
    });

    it('renders no mockup card when the mockup fence is malformed', () => {
        const fixture = TestBed.createComponent(ActivityCommentItemComponent);
        fixture.componentRef.setInput('message', designMessage('See ```mockup\n<p>unclosed'));
        fixture.componentRef.setInput('agentRun', awaitingApprovalRun());
        fixture.componentRef.setInput('isLatestPlanForStage', true);
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelectorAll('app-mockup-card').length).toBe(0);
    });
});

describe('ActivityCommentItemComponent editing (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                MarkdownModule.forRoot(),
                UiModule,
                TranslateModule.forRoot(),
                AvatarStub,
                MessageEditorStub,
                TablerIconStub,
                AttachmentLightboxStub,
                DiffViewerStub
            ],
            declarations: [ActivityCommentItemComponent, MessageBodyComponent, MockupCardComponent]
        }).compileComponents();
    });

    function renderEditing() {
        const fixture = TestBed.createComponent(ActivityCommentItemComponent);
        fixture.componentRef.setInput('message', plainMessage('old text'));
        fixture.componentRef.setInput('isOwnMessage', true);
        fixture.componentRef.setInput('isEditing', true);
        fixture.detectChanges();
        const editor = fixture.debugElement.query(By.directive(MessageEditorStub))
            .componentInstance as MessageEditorStub;
        return { fixture, editor };
    }

    it('offers save and cancel instead of send while editing', () => {
        const { editor } = renderEditing();

        expect(editor.message()).toBe('old text');
        expect(editor.sendIcon()).toBe('device-floppy');
        expect(editor.sendLabel()).toBe('SAVE');
        expect(editor.disableCancelButton()).toBe(false);
    });

    it('emits the edited text on save and closes on cancel', () => {
        const { fixture, editor } = renderEditing();
        const saved: string[] = [];
        let cancelCount = 0;
        fixture.componentInstance.editSave.subscribe(text => saved.push(text));
        fixture.componentInstance.editCancel.subscribe(() => cancelCount++);

        editor.messageChange.emit('new text');
        editor.cancelled.emit();

        expect(saved).toEqual(['new text']);
        expect(cancelCount).toBe(1);
    });
});

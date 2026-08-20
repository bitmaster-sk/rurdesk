import { Component, Pipe, PipeTransform, input, output } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MarkdownModule } from 'ngx-markdown';
import { MARKDOWN_MARKED_OPTIONS } from 'src/app/shared/markdown/marked-options';
import { TranslateModule } from '@ngx-translate/core';
import { UiModule } from 'src/app/ui/ui.module';
import { AvatarStub, MessageEditorStub, TablerIconStub } from 'src/testing/stubs';
import { ActivityCommentItemComponent } from './activity-comment-item.component';
import { MessageBodyComponent } from 'src/app/shared/mention/message-body/message-body.component';
import { MockupCardComponent } from 'src/app/shared/components/mockup-card/mockup-card.component';
import { MentionChipComponent } from 'src/app/shared/mention/mention-chip/mention-chip.component';
import { MessageKind } from 'src/app/message/constant/message-kind.enum';
import { Message } from 'src/app/message/model/message.model';
import { User } from 'src/app/auth/model/user.model';
import { AgentRun } from 'src/app/agent/model/agent-run.model';
import { AgentPhase } from 'src/app/agent/model/agent-phase.enum';

function awaitingApprovalRun(approvedMockupRef: string | null = null): AgentRun {
    return {
        idRun: 1,
        phase: AgentPhase.AwaitingApproval,
        approvedMockupRef
    } as unknown as AgentRun;
}

@Pipe({ name: 'emoji', standalone: false })
class StubEmojiPipe implements PipeTransform {
    public transform(v: string): string {
        return v;
    }
}

@Component({ selector: 'app-diff-viewer', template: '', standalone: true })
class DiffViewerStub {
    public readonly rawPatch = input<string>('');
}

@Component({ selector: 'app-anchor-reply', template: '', standalone: true })
class AnchorReplyStub {
    public readonly reply = input<unknown>(undefined);
    public readonly candidates = input<unknown>(undefined);
}

@Component({ selector: 'app-plan-actions', template: '', standalone: true })
class PlanActionsStub {
    public readonly run = input<unknown>(undefined);
    public readonly approve = output<void>();
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

// A stub anchor message so showLineMode() returns true.
function anchorChildStub(): Message {
    return {
        message: 'reply',
        messageKind: MessageKind.Comment,
        isRead: true,
        createdAt: new Date(),
        creator: { name: 'User' },
        anchor: {
            idParentMessage: 1,
            parentVersion: 1,
            anchorLineStart: 1,
            anchorLineEnd: 1,
            isOutdated: false
        }
    } as unknown as Message;
}

describe('ActivityCommentItemComponent mockup rendering (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                MarkdownModule.forRoot({ markedOptions: MARKDOWN_MARKED_OPTIONS }),
                UiModule,
                TranslateModule.forRoot(),
                AvatarStub,
                MessageEditorStub,
                TablerIconStub,
                DiffViewerStub,
                AnchorReplyStub,
                PlanActionsStub
            ],
            declarations: [
                ActivityCommentItemComponent,
                MessageBodyComponent,
                MockupCardComponent,
                MentionChipComponent,
                StubEmojiPipe
            ]
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

    it('hides the global approve and re-emits approveMockup for a design message with mockups', () => {
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

        // No single global approve control for a mockup message.
        expect(fixture.nativeElement.querySelector('app-plan-actions')).toBeFalsy();

        const btn = fixture.nativeElement.querySelector('.mockup-card__approve') as HTMLElement;
        expect(btn).toBeTruthy();
        btn.click();
        expect(ref).toBe('B #1');
    });

    it('still shows the global approve for a design message WITHOUT mockups', () => {
        const fixture = TestBed.createComponent(ActivityCommentItemComponent);
        fixture.componentRef.setInput('message', designMessage('Plain design, no mockup.'));
        fixture.componentRef.setInput('agentRun', awaitingApprovalRun());
        fixture.componentRef.setInput('isLatestPlanForStage', true);
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelector('app-plan-actions')).toBeTruthy();
    });

    it('keeps the global approve when the mockup fence is malformed (no real card rendered)', () => {
        // "```mockup" appears but the fence is never closed → the parser yields no
        // mockup segment. The global approve must NOT be suppressed, else the run
        // would be unapprovable.
        const fixture = TestBed.createComponent(ActivityCommentItemComponent);
        fixture.componentRef.setInput('message', designMessage('See ```mockup\n<p>unclosed'));
        fixture.componentRef.setInput('agentRun', awaitingApprovalRun());
        fixture.componentRef.setInput('isLatestPlanForStage', true);
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelectorAll('app-mockup-card').length).toBe(0);
        expect(fixture.nativeElement.querySelector('app-plan-actions')).toBeTruthy();
    });
});

describe('ActivityCommentItemComponent line-mode @mention rendering (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                MarkdownModule.forRoot({ markedOptions: MARKDOWN_MARKED_OPTIONS }),
                UiModule,
                TranslateModule.forRoot(),
                AvatarStub,
                MessageEditorStub,
                TablerIconStub,
                DiffViewerStub,
                AnchorReplyStub,
                PlanActionsStub
            ],
            declarations: [
                ActivityCommentItemComponent,
                MessageBodyComponent,
                MockupCardComponent,
                MentionChipComponent,
                StubEmojiPipe
            ]
        }).compileComponents();
    });

    it('renders a .mention-chip inside a line-mode line', async () => {
        const fixture = TestBed.createComponent(ActivityCommentItemComponent);
        const candidates = new Map<number, User>([
            [1, { idUser: 1, name: 'Jan', email: '', colorAvatarBg: '' }]
        ]);
        // Message with a mention token on the first line.
        fixture.componentRef.setInput('message', plainMessage('cc @[Jan](user:1) please'));
        fixture.componentRef.setInput('candidates', candidates);
        // Provide an anchor child so showLineMode() returns true.
        fixture.componentRef.setInput('anchoredChildren', [anchorChildStub()]);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        // Line mode should be active.
        const lineMode = fixture.nativeElement.querySelector('.line-mode');
        expect(lineMode).not.toBeNull();

        // The mention chip must appear inside the line-mode lines.
        const chips = fixture.nativeElement.querySelectorAll('app-mention-chip');
        expect(chips.length).toBeGreaterThan(0);
    });
});

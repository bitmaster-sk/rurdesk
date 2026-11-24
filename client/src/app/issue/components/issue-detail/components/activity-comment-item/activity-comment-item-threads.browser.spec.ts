import { Component, input } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MarkdownModule } from 'ngx-markdown';
import { TranslateModule } from '@ngx-translate/core';
import { MARKDOWN_MARKED_OPTIONS } from 'src/app/shared/markdown/marked-options';
import { MARKDOWN_MENTION_EXTENSION } from 'src/app/shared/markdown/mention-extension';
import { UiModule } from 'src/app/ui/ui.module';
import {
    AttachmentLightboxStub,
    AvatarStub,
    MessageEditorStub,
    TablerIconStub
} from 'src/testing/stubs';
import { ActivityCommentItemComponent } from './activity-comment-item.component';
import { AnchorThreadComponent } from '../anchor-thread/anchor-thread.component';
import { MessageBodyComponent } from 'src/app/shared/mention/message-body/message-body.component';
import { MessageAnchorsDirective } from 'src/app/shared/mention/message-body/message-anchors.directive';
import { MarkdownCodeCopyDirective } from 'src/app/shared/markdown/markdown-code-copy.directive';
import { MockupCardComponent } from 'src/app/shared/components/mockup-card/mockup-card.component';
import { ClipboardService } from 'src/app/core/clipboard.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { MessageKind } from 'src/app/message/constant/message-kind.enum';
import { Message } from 'src/app/message/model/message.model';
import { User } from 'src/app/auth/model/user.model';
import { AnchorDraft, AnchorThread } from 'src/app/issue/entity/anchor-thread.entity';

@Component({ selector: 'app-mermaid-diagram', template: '<svg>diagram</svg>', standalone: true })
class MermaidDiagramStub {
    public readonly source = input<string>('');
}

@Component({ selector: 'app-diff-viewer', template: '', standalone: true })
class DiffViewerStub {
    public readonly rawPatch = input<string>('');
}

@Component({ selector: 'app-plan-actions', template: '', standalone: true })
class PlanActionsStub {
    public readonly run = input<unknown>(undefined);
}

const AUTHOR: User = { idUser: 2, name: 'Agent', email: '', colorAvatarBg: '' };
const JAN: User = { idUser: 3, name: 'Jan', email: '', colorAvatarBg: '' };

function parent(body: string): Message {
    return {
        idMessage: 1,
        message: body,
        messageKind: MessageKind.Comment,
        isRead: true,
        createdAt: new Date(),
        creator: AUTHOR,
        anchor: null
    } as unknown as Message;
}

function thread(
    idThread: number,
    lineStart: number,
    lineEnd: number,
    overrides: Partial<AnchorThread> = {}
): AnchorThread {
    return {
        idThread,
        idParentMessage: 1,
        lineStart,
        lineEnd,
        anchorText: '',
        isOutdated: false,
        replies: [
            {
                idMessage: idThread,
                message: `reply ${idThread}`,
                messageKind: MessageKind.Comment,
                isRead: true,
                createdAt: new Date(),
                creator: AUTHOR
            } as unknown as Message
        ],
        ...overrides
    };
}

describe('ActivityCommentItemComponent anchored threads (browser)', () => {
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
                DiffViewerStub,
                MermaidDiagramStub,
                PlanActionsStub
            ],
            declarations: [
                ActivityCommentItemComponent,
                AnchorThreadComponent,
                MessageBodyComponent,
                MessageAnchorsDirective,
                MarkdownCodeCopyDirective,
                MockupCardComponent
            ],
            providers: [
                { provide: ClipboardService, useValue: { copy: () => Promise.resolve() } },
                { provide: ToastNotificationService, useValue: { success: () => {} } }
            ]
        }).compileComponents();
    });

    afterEach(() => window.getSelection()?.removeAllRanges());

    async function render(
        body: string,
        threads: AnchorThread[] = [],
        inputs: Record<string, unknown> = {}
    ): Promise<{ fixture: ComponentFixture<ActivityCommentItemComponent>; el: HTMLElement }> {
        const fixture = TestBed.createComponent(ActivityCommentItemComponent);
        fixture.componentRef.setInput('message', parent(body));
        fixture.componentRef.setInput('threads', threads);
        for (const [name, value] of Object.entries(inputs)) {
            fixture.componentRef.setInput(name, value);
        }
        await settle(fixture);
        document.body.appendChild(fixture.nativeElement);
        return { fixture, el: fixture.nativeElement as HTMLElement };
    }

    async function settle(fixture: ComponentFixture<unknown>): Promise<void> {
        fixture.detectChanges();
        await fixture.whenStable();
        await new Promise(resolve => setTimeout(resolve, 20));
        fixture.detectChanges();
    }

    /** Text rendered before the given element, in document order, within the comment. */
    function textBefore(el: HTMLElement, target: Element): string {
        const range = document.createRange();
        range.setStartBefore(el.querySelector('.body-host')!);
        range.setEndBefore(target);
        return range.toString();
    }

    function textNode(el: HTMLElement, text: string): Text {
        const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
        for (let node = walker.nextNode(); node; node = walker.nextNode()) {
            if (node.textContent?.includes(text)) {
                return node as Text;
            }
        }
        throw new Error(`no text node with "${text}"`);
    }

    async function select(
        fixture: ComponentFixture<unknown>,
        el: HTMLElement,
        from: [string, string],
        to: [string, string]
    ): Promise<HTMLElement | null> {
        const startNode = textNode(el, from[0]);
        const endNode = textNode(el, to[0]);
        const range = document.createRange();
        range.setStart(startNode, startNode.data.indexOf(from[1]));
        range.setEnd(endNode, endNode.data.indexOf(to[1]) + to[1].length);
        const selection = window.getSelection()!;
        selection.removeAllRanges();
        selection.addRange(range);
        el.querySelector('.body-host')!.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
        fixture.detectChanges();
        return el.querySelector<HTMLElement>('[data-testid="anchor-selection-reply"]');
    }

    async function requestFrom(
        fixture: ComponentFixture<ActivityCommentItemComponent>,
        el: HTMLElement,
        from: [string, string],
        to: [string, string]
    ): Promise<AnchorDraft | null> {
        let requested: AnchorDraft | null = null;
        fixture.componentInstance.anchorRequest.subscribe(draft => (requested = draft));
        const button = await select(fixture, el, from, to);
        button?.click();
        return requested;
    }

    describe('a parent with threads', () => {
        it('renders a thread right after the block that ends its range', async () => {
            const { el } = await render('# Plan\n\nFirst para\n\nSecond para\n\nThird para', [
                thread(5, 3, 3)
            ]);
            const threadEl = el.querySelector('app-anchor-thread')!;

            expect(textBefore(el, threadEl)).toContain('First para');
            expect(textBefore(el, threadEl)).not.toContain('Second para');
        });

        it('keeps list numbering and code blocks intact across the split', async () => {
            const body = '1. one\n2. two\n3. three\n4. four\n5. five\n\n```\ncode line\n```';
            const { el } = await render(body, [thread(5, 3, 3)]);

            expect(el.querySelector('ol[start="4"]')?.textContent).toContain('four');
            expect(el.querySelectorAll('ol li')).toHaveLength(5);
            expect(el.querySelector('pre code')?.textContent).toContain('code line');
        });

        it('wraps the quote in a mark, or highlights the whole range without one', async () => {
            const { el } = await render('alpha beta gamma\n\nsecond block', [
                thread(5, 1, 1, { quote: 'beta' }),
                thread(6, 3, 3)
            ]);

            const mark = el.querySelector('mark[data-anchor-thread="5"]');
            expect(mark?.textContent).toBe('beta');
            const block = el.querySelector('.anchor-highlight[data-anchor-thread="6"]');
            expect(block?.textContent).toContain('second block');
        });

        it('highlights the quote in a part whose blocks cannot be told apart', async () => {
            const { el } = await render('<!-- note -->\n\nalpha beta gamma', [
                thread(5, 3, 3, { quote: 'beta' })
            ]);

            expect(el.querySelector('mark[data-anchor-thread="5"]')?.textContent).toBe('beta');
        });

        it('keeps two threads in one paragraph apart, each opening its own', async () => {
            const { fixture, el } = await render('alpha beta gamma', [
                thread(5, 1, 1, { quote: 'alpha' }),
                thread(6, 1, 1, { quote: 'gamma' })
            ]);
            const toggled: number[] = [];
            fixture.componentInstance.threadToggle.subscribe(id => toggled.push(id));

            expect(el.querySelectorAll('app-anchor-thread')).toHaveLength(2);
            el.querySelector<HTMLElement>('mark[data-anchor-thread="6"]')!.click();
            el.querySelector<HTMLElement>('mark[data-anchor-thread="5"]')!.click();

            expect(toggled).toEqual([6, 5]);
        });

        it('shows an outdated thread with its anchor text in the older-version section, unhighlighted', async () => {
            const { el } = await render('current text', [
                thread(5, 1, 1, { isOutdated: true, anchorText: 'removed text' })
            ]);
            const section = el.querySelector('[data-testid="anchor-outdated-section"]')!;

            expect(section.textContent).toContain('removed text');
            expect(section.querySelector('app-anchor-thread')).not.toBeNull();
            expect(el.querySelector('[data-anchor-thread]')).toBeNull();
        });

        it('keeps the highlight after the parent text is re-rendered', async () => {
            const { fixture, el } = await render('alpha beta', [
                thread(5, 1, 1, { quote: 'beta' })
            ]);
            fixture.componentRef.setInput('message', parent('new line\nalpha beta'));
            fixture.componentRef.setInput('threads', [thread(5, 2, 2, { quote: 'beta' })]);
            await settle(fixture);

            expect(el.querySelector('mark[data-anchor-thread="5"]')?.textContent).toBe('beta');
        });

        it('renders mention chips in a split body', async () => {
            const { fixture, el } = await render('cc @[Jan](user:3)\n\nnext', [thread(5, 1, 1)], {
                candidates: new Map([[3, JAN]])
            });
            await settle(fixture);

            expect(el.querySelectorAll('.mention-chip').length).toBeGreaterThan(0);
        });
    });

    describe('placing a thread under its text', () => {
        const WORDS = Array.from({ length: 60 }, (_, index) => `word${index}`).join(' ');

        async function renderAt(
            width: number,
            quote: string
        ): Promise<{ fixture: ComponentFixture<ActivityCommentItemComponent>; el: HTMLElement }> {
            const page = await render(WORDS, [thread(5, 1, 1, { quote })]);
            page.el.style.display = 'block';
            page.el.style.width = `${width}px`;
            await layout();
            return page;
        }

        async function resize(el: HTMLElement, width: number): Promise<void> {
            el.style.width = `${width}px`;
            await layout();
        }

        /** Lets the resize observer run and the browser lay the page out again. */
        async function layout(): Promise<void> {
            await new Promise(resolve => requestAnimationFrame(resolve));
            await new Promise(resolve => setTimeout(resolve, 30));
        }

        function rectOf(el: HTMLElement, text: string): DOMRect {
            const node = textNode(el, text);
            const range = document.createRange();
            range.setStart(node, node.data.indexOf(text));
            range.setEnd(node, node.data.indexOf(text) + text.length);
            return range.getBoundingClientRect();
        }

        function expectRightUnderTheQuote(el: HTMLElement): void {
            const marks = el.querySelectorAll('mark[data-anchor-thread="5"]');
            const quoteRect = marks[marks.length - 1].getBoundingClientRect();
            const threadRect = el.querySelector('.anchor-thread')!.getBoundingClientRect();

            expect(threadRect.top).toBeGreaterThanOrEqual(quoteRect.bottom - 1);
            expect(threadRect.top - quoteRect.bottom).toBeLessThan(16);
            expect(rectOf(el, 'word59').top).toBeGreaterThanOrEqual(threadRect.bottom);
        }

        afterEach(() =>
            document.querySelectorAll('app-activity-comment-item').forEach(el => el.remove())
        );

        it('puts the thread right under the line its quote ends on, the rest of the text below it', async () => {
            const { el } = await renderAt(320, 'word3');

            expectRightUnderTheQuote(el);
        });

        it('points the arrow at the end of the quote', async () => {
            const { el } = await renderAt(320, 'word10');
            const quoteRect = el
                .querySelector('mark[data-anchor-thread="5"]')!
                .getBoundingClientRect();
            const threadEl = el.querySelector<HTMLElement>('.anchor-thread')!;
            const arrowLeft = parseFloat(threadEl.style.getPropertyValue('--anchor-arrow-left'));

            expect(
                Math.abs(threadEl.getBoundingClientRect().left + arrowLeft + 14 - quoteRect.right)
            ).toBeLessThan(2);
        });

        it('follows the quote when the text wraps at another width', async () => {
            const { el } = await renderAt(320, 'word20');
            const topBefore = el.querySelector('.anchor-thread')!.getBoundingClientRect().top;

            await resize(el, 200);

            expect(el.querySelector('.anchor-thread')!.getBoundingClientRect().top).not.toBe(
                topBefore
            );
            expectRightUnderTheQuote(el);
        });

        it('makes room for the replies when the thread is opened', async () => {
            const { fixture, el } = await renderAt(320, 'word3');
            const heightBefore = el.querySelector('.anchor-thread')!.getBoundingClientRect().height;

            fixture.componentRef.setInput('expandedThreads', new Set([5]));
            await settle(fixture);
            await layout();

            expect(
                el.querySelector('.anchor-thread')!.getBoundingClientRect().height
            ).toBeGreaterThan(heightBefore);
            expectRightUnderTheQuote(el);
        });

        it('leaves the thread after the paragraph when the quote is on its last line', async () => {
            const { el } = await renderAt(320, 'word59');

            expect(el.querySelector('.anchor-gap')).toBeNull();
            expect(
                el.querySelector('.anchor-thread')!.getBoundingClientRect().top
            ).toBeGreaterThanOrEqual(rectOf(el, 'word59').bottom - 1);
        });

        it('keeps a later overlaid thread aligned when an earlier thread expands', async () => {
            const secondPara = Array.from({ length: 60 }, (_, index) => `next${index}`).join(' ');
            const { fixture, el } = await render(`${WORDS}\n\n${secondPara}`, [
                thread(5, 1, 1, { quote: 'word3' }),
                thread(6, 3, 3, { quote: 'next3' })
            ]);
            el.style.display = 'block';
            el.style.width = '320px';
            await layout();

            fixture.componentRef.setInput('expandedThreads', new Set([5]));
            await settle(fixture);
            await layout();

            const secondQuote = el
                .querySelector('mark[data-anchor-thread="6"]')!
                .getBoundingClientRect();
            const secondThread = el.querySelector('[data-thread="6"]')!.getBoundingClientRect();

            expect(secondThread.top).toBeGreaterThanOrEqual(secondQuote.bottom - 1);
            expect(secondThread.top - secondQuote.bottom).toBeLessThan(16);
            expect(rectOf(el, 'next59').top).toBeGreaterThanOrEqual(secondThread.bottom);
        });

        it('keeps a following single-paragraph chunk on its own block line when the previous slot overlays', async () => {
            const { el } = await render(`${WORDS}\n\nTrailing paragraph`, [
                thread(5, 1, 1, { quote: 'word3' })
            ]);
            el.style.display = 'block';
            el.style.width = '320px';
            await layout();

            expect(rectOf(el, 'Trailing paragraph').top).toBeGreaterThanOrEqual(
                rectOf(el, 'word59').bottom - 1
            );
        });
    });

    describe('replying to a selection', () => {
        it('anchors a part of a paragraph to the whole paragraph', async () => {
            const { fixture, el } = await render(
                '# Plan\n\nFirst line of text\nsecond line\n\nAfter'
            );
            const draft = await requestFrom(
                fixture,
                el,
                ['First line', 'line'],
                ['First line', 'text']
            );

            expect(draft).toEqual({
                idParentMessage: 1,
                lineStart: 3,
                lineEnd: 4,
                quote: 'line of text'
            });
        });

        it('covers both items of a selection across a list', async () => {
            const { fixture, el } = await render('Intro\n\n- apple pie\n- banana split\n- cherry');
            const draft = await requestFrom(
                fixture,
                el,
                ['apple pie', 'pie'],
                ['banana split', 'banana']
            );

            expect(draft?.lineStart).toBe(3);
            expect(draft?.lineEnd).toBe(4);
            expect(draft?.quote).toContain('pie');
            expect(draft?.quote).toContain('banana');
        });

        it('anchors a selection inside a code block to the block, without the copy control', async () => {
            const { fixture, el } = await render('Before\n\n```\nconst a = 1;\n```');
            const draft = await requestFrom(fixture, el, ['Before', 'Before'], ['const a', '1;']);

            expect(draft?.lineStart).toBe(1);
            expect(draft?.lineEnd).toBe(5);
            expect(draft?.quote).toBe('Before const a = 1;');
        });

        it('reads the visible text of mention chips and links', async () => {
            const { fixture, el } = await render(
                'ask @[Jan](user:3) about [the docs](https://x.test) now',
                [],
                {
                    candidates: new Map([[3, JAN]])
                }
            );
            const draft = await requestFrom(fixture, el, ['ask', 'ask'], ['now', 'now']);

            expect(draft?.quote).toContain('Jan');
            expect(draft?.quote).toContain('the docs');
        });

        it('offers no reply when the selection touches a diagram', async () => {
            const { fixture, el } = await render('Before\n\n```mermaid\ngraph TD\n```\n\nAfter');
            const button = await select(fixture, el, ['Before', 'Before'], ['After', 'After']);

            expect(button).toBeNull();
        });

        it('offers no reply for a selection inside a thread', async () => {
            const { fixture, el } = await render('Plan text', [thread(5, 1, 1)], {
                expandedThreads: new Set([5])
            });
            const button = await select(fixture, el, ['reply 5', 'reply'], ['reply 5', '5']);

            expect(button).toBeNull();
        });

        it('offers a reply when the drag ends outside the message', async () => {
            const { fixture, el } = await render('First line of text');
            const node = textNode(el, 'First line');
            const range = document.createRange();
            range.setStart(node, 0);
            range.setEnd(node, 'First'.length);
            window.getSelection()!.removeAllRanges();
            window.getSelection()!.addRange(range);
            document.body.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
            fixture.detectChanges();

            expect(el.querySelector('[data-testid="anchor-selection-reply"]')).not.toBeNull();
        });

        it('removes the reply button once the selection moves to other text', async () => {
            const { fixture, el } = await render('First line of text');
            expect(await select(fixture, el, ['First', 'First'], ['First', 'line'])).not.toBeNull();

            const other = document.createElement('p');
            other.textContent = 'Other message';
            document.body.appendChild(other);
            const range = document.createRange();
            range.selectNodeContents(other);
            window.getSelection()!.removeAllRanges();
            window.getSelection()!.addRange(range);
            other.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
            fixture.detectChanges();
            other.remove();

            expect(el.querySelector('[data-testid="anchor-selection-reply"]')).toBeNull();
        });

        it('opens the draft composer under the range and closes it on Escape', async () => {
            const draft: AnchorDraft = {
                idParentMessage: 1,
                lineStart: 1,
                lineEnd: 1,
                quote: 'First'
            };
            const { fixture, el } = await render('First\n\nSecond', [], { draft });
            let cancelled = 0;
            fixture.componentInstance.draftCancel.subscribe(() => cancelled++);
            const composer = el.querySelector('[data-testid="anchor-thread-draft"]')!;

            expect(textBefore(el, composer)).not.toContain('Second');
            composer.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));

            expect(cancelled).toBe(1);
        });
    });
});

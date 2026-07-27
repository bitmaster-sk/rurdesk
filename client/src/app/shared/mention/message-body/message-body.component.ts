import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { User } from 'src/app/auth/model/user.model';
import { MessageKind } from 'src/app/message/constant/message-kind.enum';
import { extractMessageSegments } from 'src/app/shared/mention/extract-message-segments';
import { parseMentionParts } from 'src/app/shared/mention/mention-token.util';

export type RenderSegment =
    | { type: 'diff'; content: string }
    | { type: 'mockup'; content: string; title?: string; ref: string }
    | { type: 'text'; content: string }
    | { type: 'mention'; idUser: number; name: string };

/** Span within a text block — either a code region or plain text. */
interface TextSpan {
    isCode: boolean;
    text: string;
}

/**
 * Split raw markdown text into alternating code / non-code spans so that
 * mention tokens inside fenced blocks (``` … ```) or inline code (` … `) are
 * never parsed as mentions — they must render literally.
 *
 * Handles:
 *  - Fenced blocks opened with three (or more) backticks, closed by a matching
 *    fence (same length). Multi-line.
 *  - Inline code enclosed by one or more backticks (not newline-crossing by
 *    the CommonMark spec, but we're lenient here).
 */
export function splitCodeSpans(text: string): TextSpan[] {
    const spans: TextSpan[] = [];
    let i = 0;

    while (i < text.length) {
        if (text[i] !== '`') {
            const next = text.indexOf('`', i);
            if (next === -1) {
                spans.push({ isCode: false, text: text.slice(i) });
                break;
            }
            spans.push({ isCode: false, text: text.slice(i, next) });
            i = next;
            continue;
        }

        let fenceLen = 0;
        while (i + fenceLen < text.length && text[i + fenceLen] === '`') {
            fenceLen++;
        }
        const opener = text.slice(i, i + fenceLen);
        const closerStart = text.indexOf(opener, i + fenceLen);
        if (closerStart === -1) {
            // No closer — treat the rest as plain text (no code).
            spans.push({ isCode: false, text: text.slice(i) });
            break;
        }
        // Ensure the closer is not immediately followed by another backtick
        // (which would make it a longer fence, not a match).
        const afterCloser = closerStart + fenceLen;
        if (afterCloser < text.length && text[afterCloser] === '`') {
            // Not a proper close — skip past opener and keep scanning.
            spans.push({ isCode: false, text: opener });
            i = i + fenceLen;
            continue;
        }
        const codeContent = text.slice(i, afterCloser);
        spans.push({ isCode: true, text: codeContent });
        i = afterCloser;
    }

    return spans;
}

const AGENT_KINDS = new Set<MessageKind>([
    MessageKind.Design,
    MessageKind.ImplementationPlan,
    MessageKind.BrainstormingQuestion
]);

@Component({
    selector: 'app-message-body',
    templateUrl: './message-body.component.html',
    styleUrls: ['./message-body.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class MessageBodyComponent {
    public readonly body = input.required<string>();
    public readonly messageKind = input<MessageKind | undefined>(undefined);
    public readonly candidates = input<Map<number, User> | User[] | null>(null);

    /** When true, mockup cards in this body show the "use & approve" action. */
    public readonly approvable = input(false);
    /** The approved mockup's ref (from the run); drives selected/rejected badges. */
    public readonly selectedMockupRef = input<string | null>(null);
    /** Emits the ref of the mockup the user chose to approve. */
    public readonly useMockup = output<string>();

    public readonly segments = computed<RenderSegment[]>(() => {
        const kind = this.messageKind();
        const isAgentKind = kind !== undefined && AGENT_KINDS.has(kind);

        // For agent messages: split into text/diff/mockup via extractMessageSegments.
        // For user messages: a single text segment (no diff/mockup splitting).
        const base = isAgentKind
            ? extractMessageSegments(this.body())
            : [{ type: 'text' as const, content: this.body() }];

        const out: RenderSegment[] = [];

        for (const seg of base) {
            if (seg.type === 'mockup') {
                out.push({
                    type: 'mockup',
                    content: seg.content,
                    title: seg.title,
                    ref: seg.ref ?? ''
                });
                continue;
            }
            if (seg.type !== 'text') {
                out.push(seg as RenderSegment);
                continue;
            }

            if (isAgentKind) {
                // Agent text segments: parse mention tokens directly (no code-fence guard
                // needed — agent messages aren't user-typed fenced code, and their
                // diff/mockup blocks were already split off above).
                for (const p of parseMentionParts(seg.content)) {
                    if (p.type === 'text') {
                        if (p.text) out.push({ type: 'text', content: p.text });
                    } else {
                        out.push({ type: 'mention', idUser: p.idUser, name: p.name });
                    }
                }
            } else {
                // User-typed message: guard against mentions inside code fences / inline code.
                const codeSpans = splitCodeSpans(seg.content);
                for (const span of codeSpans) {
                    if (span.isCode) {
                        // Emit the raw code text as-is (literal, no chip).
                        if (span.text) out.push({ type: 'text', content: span.text });
                    } else {
                        // Non-code span: parse mentions.
                        for (const p of parseMentionParts(span.text)) {
                            if (p.type === 'text') {
                                if (p.text) out.push({ type: 'text', content: p.text });
                            } else {
                                out.push({ type: 'mention', idUser: p.idUser, name: p.name });
                            }
                        }
                    }
                }
            }
        }

        return out;
    });
}

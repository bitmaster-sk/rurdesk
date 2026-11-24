import {
    ChangeDetectionStrategy,
    Component,
    TemplateRef,
    computed,
    effect,
    inject,
    input,
    output
} from '@angular/core';
import { Router } from '@angular/router';
import { User } from 'src/app/auth/model/user.model';
import { AttachmentGallery } from 'src/app/shared/attachment/service/attachment-gallery.service';
import { AttachmentLinkConverter } from 'src/app/shared/attachment/converter/attachment-link.converter';
import { MessageKind } from 'src/app/message/constant/message-kind.enum';
import {
    LinedMessageSegment,
    MessageSegmentParser
} from 'src/app/shared/mention/message-segment.parser';
import { AnchorChunk, AnchorChunker } from 'src/app/shared/mention/anchor-chunker';
import { AnchorHighlight } from './anchor-dom';
import { MENTION_TOKEN_RE } from 'src/app/shared/mention/mention';
import { LinkScope } from 'src/app/link/service/link-scope.service';
import { WikiLinkConverter } from 'src/app/wiki/converter/wiki-link.converter';
import { WikiLinkStore } from 'src/app/wiki/store/wiki-link.store';

/** Source lines of a rendered piece and the thread ranges that end in it. */
interface SegmentLines {
    lineStart: number;
    lineEnd: number;
    breaks: number[];
}

export type RenderSegment = SegmentLines &
    (
        | { type: 'diff'; content: string }
        | { type: 'mockup'; content: string; title?: string; ref: string }
        | { type: 'mermaid'; content: string }
        | { type: 'plantuml'; content: string }
        | { type: 'text'; content: string }
    );

/** Context of the anchor slot template: the thread end lines that render at this spot. */
export interface AnchorSlotContext {
    $implicit: number[];
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
    providers: [AttachmentGallery],
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

    /** End lines of thread ranges; the text is split after the block holding each one. */
    public readonly anchorBreaks = input<number[]>([]);
    /** Rendered after a piece of the body for the thread ranges that end in it. */
    public readonly anchorSlot = input<TemplateRef<AnchorSlotContext> | null>(null);
    public readonly anchorHighlights = input<AnchorHighlight[]>([]);
    public readonly anchorClick = output<number>();

    protected readonly gallery = inject(AttachmentGallery);
    private readonly router = inject(Router);
    private readonly linkScope = inject(LinkScope, { optional: true });
    private readonly wikiStore = inject(WikiLinkStore);

    private readonly wikiContext = computed(() => {
        const idProject = this.linkScope?.idProject() ?? null;
        const tree = idProject === null ? null : this.wikiStore.tree(idProject);
        return idProject !== null && tree ? WikiLinkConverter.toContext(idProject, tree) : null;
    });

    public readonly segments = computed<RenderSegment[]>(() => {
        const context = this.wikiContext();
        return this.baseSegments().map(segment => {
            if (segment.type !== 'text') {
                return segment;
            }
            let content = segment.content;
            if (context && content.includes('[[')) {
                content = WikiLinkConverter.toPageLinksMarkdown(content, context);
            }
            if (content.includes('](attachment:')) {
                content = AttachmentLinkConverter.toMarkerMarkdown(content);
            }
            return content === segment.content ? segment : { ...segment, content };
        });
    });

    /** Highlights of the threads whose range touches each piece, by piece index. */
    protected readonly highlightsBySegment = computed<AnchorHighlight[][]>(() => {
        const highlights = this.anchorHighlights();
        return this.segments().map(segment =>
            highlights.filter(
                highlight =>
                    highlight.lineStart <= segment.lineEnd && highlight.lineEnd >= segment.lineStart
            )
        );
    });

    public constructor() {
        effect(() => {
            const idProject = this.linkScope?.idProject() ?? null;
            if (idProject !== null && this.body().includes('[[')) {
                this.wikiStore.ensure(idProject);
            }
        });
    }

    protected onTextClick(event: MouseEvent): void {
        const anchor = (event.target as HTMLElement | null)?.closest('a');
        const href = anchor?.getAttribute('href');
        if (!href?.startsWith('/') || event.metaKey || event.ctrlKey || event.shiftKey) {
            return;
        }
        event.preventDefault();
        event.stopPropagation();
        void this.router.navigateByUrl(href);
    }

    private readonly baseSegments = computed<RenderSegment[]>(() => {
        const kind = this.messageKind();
        const isAgentKind = kind !== undefined && AGENT_KINDS.has(kind);

        // Agent messages split diff/mockup/mermaid/plantuml fences; user messages
        // split diagram fences (mermaid, plantuml) only — diff and mockup stay
        // plain code blocks.
        const base = isAgentKind
            ? MessageSegmentParser.parseLines(this.body())
            : MessageSegmentParser.parseLines(this.body(), 'diagrams');

        const out: RenderSegment[] = [];
        let pending = [...this.anchorBreaks()].sort((a, b) => a - b);

        base.forEach((seg, index) => {
            // A piece takes the breaks up to its last line; the final piece takes any left over.
            const isLast = index === base.length - 1;
            const owned = pending.filter(line => isLast || line <= seg.lineEnd);
            pending = pending.filter(line => !owned.includes(line));

            if (seg.type === 'text') {
                // Split on the raw source, then resolve mentions per piece: the rewrites keep
                // lines intact, so piece lines stay valid.
                for (const chunk of this.chunks(seg, owned)) {
                    out.push({
                        ...chunk,
                        type: 'text',
                        content: this.resolveMentionNames(chunk.content)
                    });
                }
                return;
            }
            const lines: SegmentLines = {
                lineStart: seg.lineStart,
                lineEnd: seg.lineEnd,
                breaks: owned
            };
            if (seg.type === 'mockup') {
                out.push({
                    type: 'mockup',
                    content: seg.content,
                    title: seg.title,
                    ref: seg.ref ?? '',
                    ...lines
                });
                return;
            }
            out.push({ type: seg.type, content: seg.content, ...lines });
        });

        return out;
    });

    private chunks(segment: LinedMessageSegment, breaks: number[]): AnchorChunk[] {
        if (breaks.length === 0) {
            return [
                {
                    content: segment.content,
                    lineStart: segment.lineStart,
                    lineEnd: segment.lineEnd,
                    breaks
                }
            ];
        }
        return AnchorChunker.split(segment.content, segment.lineStart, breaks);
    }

    // Replace stored mention names with live candidate names so chips show the
    // current user name. Same lookup logic as the old MentionChipComponent.displayName.
    private resolveMentionNames(text: string): string {
        const cands = this.candidates();
        if (!cands) return text;
        return text.replace(MENTION_TOKEN_RE, (_match, name: string, idStr: string) => {
            const idUser = Number(idStr);
            const found =
                cands instanceof Map ? cands.get(idUser) : cands.find(u => u.idUser === idUser);
            const liveName = found?.name;
            if (!liveName || liveName.includes(']')) return _match;
            return `@[${liveName}](user:${idUser})`;
        });
    }
}

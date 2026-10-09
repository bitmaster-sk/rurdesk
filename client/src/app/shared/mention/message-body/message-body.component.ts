import {
    ChangeDetectionStrategy,
    Component,
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
import { MessageSegmentParser } from 'src/app/shared/mention/message-segment.parser';
import { MENTION_TOKEN_RE } from 'src/app/shared/mention/mention';
import { WikiLinkConverter } from 'src/app/wiki/converter/wiki-link.converter';
import { WikiLinkScope } from 'src/app/wiki/service/wiki-link-scope.service';
import { WikiLinkStore } from 'src/app/wiki/store/wiki-link.store';

export type RenderSegment =
    | { type: 'diff'; content: string }
    | { type: 'mockup'; content: string; title?: string; ref: string }
    | { type: 'text'; content: string };

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

    protected readonly gallery = inject(AttachmentGallery);
    private readonly router = inject(Router);
    private readonly wikiScope = inject(WikiLinkScope, { optional: true });
    private readonly wikiStore = inject(WikiLinkStore);

    private readonly wikiContext = computed(() => {
        const idProject = this.wikiScope?.idProject() ?? null;
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
            return content === segment.content ? segment : { type: 'text', content };
        });
    });

    public constructor() {
        effect(() => {
            const idProject = this.wikiScope?.idProject() ?? null;
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

        // For agent messages: split into text/diff/mockup via MessageSegmentParser.parse.
        // For user messages: a single text segment (no diff/mockup splitting).
        const base = isAgentKind
            ? MessageSegmentParser.parse(this.body())
            : [{ type: 'text' as const, content: this.resolveMentionNames(this.body()) }];

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
            if (seg.type === 'diff') {
                out.push({ type: 'diff', content: seg.content });
                continue;
            }

            // Text segment: resolve live mention names, emit as single markdown document.
            out.push({ type: 'text', content: this.resolveMentionNames(seg.content) });
        }

        return out;
    });

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

import {
    DestroyRef,
    Directive,
    ElementRef,
    effect,
    inject,
    input,
    output,
    untracked
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MarkdownComponent } from 'ngx-markdown';
import { MarkdownBlockMap } from 'src/app/shared/mention/markdown-block-map';
import {
    ANCHOR_LINE_START,
    ANCHOR_THREAD,
    AnchorDom,
    AnchorHighlight
} from 'src/app/shared/mention/message-body/anchor-dom';
import { AnchorSlotLayout } from 'src/app/shared/mention/message-body/anchor-slot-layout';

const HIGHLIGHT_CLASS = 'anchor-highlight';

/**
 * Annotates rendered Markdown blocks with source lines and highlights each thread's anchored text.
 * Defers one tick after `ready` so sibling directives finish rewriting the DOM first.
 */
@Directive({
    selector: '[appMessageAnchors]',
    standalone: false
})
export class MessageAnchorsDirective {
    public readonly anchorContent = input.required<string>();
    public readonly anchorLineStart = input.required<number>();
    public readonly anchorLineEnd = input.required<number>();
    public readonly anchorHighlights = input<AnchorHighlight[]>([]);
    /** Where the threads of this chunk render; they are moved under the line their text ends on. */
    public readonly anchorSlotHost = input<HTMLElement | null>(null);

    public readonly anchorClick = output<number>();

    private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
    private readonly destroyRef = inject(DestroyRef);
    private timer: ReturnType<typeof setTimeout> | null = null;
    // A new width rewraps the text and an opened thread grows, so either moves the gap.
    private readonly slotResize = new ResizeObserver(() => this.layoutSlot());

    public constructor() {
        inject(MarkdownComponent, { self: true })
            .ready.pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(() => this.schedule());
        effect(() => {
            this.anchorHighlights();
            untracked(() => this.schedule());
        });
        effect(() => {
            const slot = this.anchorSlotHost();
            this.slotResize.disconnect();
            if (slot) {
                this.slotResize.observe(slot);
            }
        });
        this.host.nativeElement.addEventListener('click', this.onClick);
        this.destroyRef.onDestroy(() => {
            this.slotResize.disconnect();
            this.host.nativeElement.removeEventListener('click', this.onClick);
            if (this.timer !== null) {
                clearTimeout(this.timer);
            }
        });
    }

    private readonly onClick = (event: MouseEvent): void => {
        const marked = (event.target as Element | null)?.closest(`[${ANCHOR_THREAD}]`);
        if (marked && this.host.nativeElement.contains(marked)) {
            this.anchorClick.emit(Number(marked.getAttribute(ANCHOR_THREAD)));
        }
    };

    private schedule(): void {
        if (this.timer !== null) {
            clearTimeout(this.timer);
        }
        this.timer = setTimeout(() => {
            this.timer = null;
            AnchorSlotLayout.reset(this.host.nativeElement, this.anchorSlotHost());
            this.annotateBlocks();
            this.clearHighlights();
            for (const highlight of this.anchorHighlights()) {
                this.highlight(highlight);
            }
            this.layoutSlot();
        });
    }

    private layoutSlot(): void {
        const slot = this.anchorSlotHost();
        if (slot) {
            AnchorSlotLayout.apply(this.host.nativeElement, slot);
        }
    }

    /**
     * Top-level children map 1:1 to the blocks that render; when they do not (an html block the
     * sanitizer changed), only the chunk as a whole carries lines.
     */
    private annotateBlocks(): void {
        const host = this.host.nativeElement;
        const blocks = MarkdownBlockMap.map(this.anchorContent(), this.anchorLineStart()).filter(
            block => block.hasDom
        );
        // The chunk may start on the newline that ends the previous line; its text starts later.
        AnchorDom.setLines(
            host,
            blocks[0]?.lineStart ?? this.anchorLineStart(),
            blocks.at(-1)?.lineEnd ?? this.anchorLineEnd()
        );
        const children = Array.from(host.children);
        if (children.length !== blocks.length) {
            return;
        }
        blocks.forEach((block, index) => {
            const element = children[index];
            AnchorDom.setLines(element, block.lineStart, block.lineEnd);
            const list = element.matches('ol, ul') ? element : null;
            const items = list ? Array.from(list.children) : [];
            if (items.length === block.items.length) {
                block.items.forEach((item, itemIndex) =>
                    AnchorDom.setLines(items[itemIndex], item.lineStart, item.lineEnd)
                );
            }
        });
    }

    private clearHighlights(): void {
        const host = this.host.nativeElement;
        for (const mark of Array.from(host.querySelectorAll(`mark[${ANCHOR_THREAD}]`))) {
            const parent = mark.parentNode;
            if (!parent) {
                continue;
            }
            while (mark.firstChild) {
                parent.insertBefore(mark.firstChild, mark);
            }
            parent.removeChild(mark);
            parent.normalize();
        }
        for (const element of [host, ...Array.from(host.querySelectorAll(`.${HIGHLIGHT_CLASS}`))]) {
            element.classList.remove(HIGHLIGHT_CLASS);
            element.removeAttribute(ANCHOR_THREAD);
        }
    }

    private highlight(highlight: AnchorHighlight): void {
        const blocks = this.blocksOf(highlight);
        if (blocks.length === 0) {
            return;
        }
        if (highlight.quote && AnchorDom.wrapQuote(blocks, highlight.quote, highlight.idThread)) {
            return;
        }
        for (const block of blocks) {
            block.classList.add(HIGHLIGHT_CLASS);
            block.setAttribute(ANCHOR_THREAD, String(highlight.idThread));
        }
    }

    /**
     * The outermost line-marked blocks inside the range, else the ones it overlaps. When only the
     * chunk carries lines, the chunk is the one block.
     */
    private blocksOf(highlight: AnchorHighlight): HTMLElement[] {
        const host = this.host.nativeElement;
        const descendants = Array.from(
            host.querySelectorAll<HTMLElement>(`[${ANCHOR_LINE_START}]`)
        );
        const marked = (descendants.length > 0 ? descendants : [host]).map(element => ({
            element,
            ...AnchorDom.linesOf(element)
        }));
        const inside = marked.filter(
            block => block.lineStart >= highlight.lineStart && block.lineEnd <= highlight.lineEnd
        );
        const candidates =
            inside.length > 0
                ? inside
                : marked.filter(
                      block =>
                          block.lineStart <= highlight.lineEnd &&
                          block.lineEnd >= highlight.lineStart
                  );
        const elements = candidates.map(block => block.element);
        // Inside the range the outermost blocks cover everything. An overlap is narrowed to the
        // deepest blocks, or the chunk host alone would highlight the whole chunk.
        return elements.filter(element =>
            elements.every(
                other =>
                    other === element ||
                    !(inside.length > 0 ? other.contains(element) : element.contains(other))
            )
        );
    }
}

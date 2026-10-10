import { DestroyRef, Directive, ElementRef, effect, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { IconFile, IconFileTypePdf } from '@tabler/icons-angular';
import { MarkdownComponent } from 'ngx-markdown';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { UiTablerSvg } from 'src/app/ui/util/ui-tabler-svg';
import { AttachmentGallery } from '../service/attachment-gallery.service';
import { AttachmentLinkConverter } from '../converter/attachment-link.converter';
import { AttachmentState } from '../entity/attachment-state.entity';
import { AttachmentStore } from '../store/attachment.store';

interface AttachmentSlot {
    idAttachment: string;
    name: string;
    isImage: boolean;
    host: HTMLElement;
    rendered?: AttachmentState | null;
}

@Directive({
    selector: '[appAttachmentRender]',
    standalone: false
})
export class AttachmentRenderDirective {
    private static readonly boxClass = 'm-1 rounded-[var(--ui-radius-md)] border ui-bg-surface-100';
    private static readonly placeholderClass =
        'inline-block rounded-[var(--ui-radius-md)] border !border-dashed px-3 py-2 ui-color-text-muted';
    private static readonly thumbWidth = '160px';
    private static readonly thumbHeight = '120px';
    private static readonly thumbClass = `${AttachmentRenderDirective.boxClass} inline-block cursor-zoom-in overflow-hidden p-0 align-top`;
    private static readonly cardClass = `${AttachmentRenderDirective.boxClass} inline-flex max-w-[320px] items-center gap-2 px-3 py-2 no-underline ui-color-text`;

    private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
    private readonly store = inject(AttachmentStore);
    private readonly i18n = inject(I18nService);
    private readonly gallery = inject(AttachmentGallery, { optional: true });
    private readonly slots = signal<AttachmentSlot[]>([]);

    public constructor() {
        inject(DestroyRef).onDestroy(() => {
            this.gallery?.unregister(this);
            this.releaseSlots();
        });
        inject(MarkdownComponent, { self: true })
            .ready.pipe(takeUntilDestroyed(inject(DestroyRef)))
            .subscribe(() => this.collect());
        effect(() => {
            for (const slot of this.slots()) {
                const state = this.store.state(slot.idAttachment);
                if (slot.rendered !== state) {
                    slot.rendered = state;
                    this.render(slot, state);
                }
            }
        });
    }

    private collect(): void {
        const previous = this.slots();
        const anchors = Array.from(
            this.host.nativeElement.querySelectorAll<HTMLAnchorElement>('a[title^="attachment-"]')
        );
        const slots: AttachmentSlot[] = [];
        for (const anchor of anchors) {
            const idAttachment = AttachmentLinkConverter.toIdAttachment(
                anchor.getAttribute('href') ?? ''
            );
            if (!idAttachment) {
                continue;
            }
            const host = document.createElement('span');
            anchor.replaceWith(host);
            slots.push({
                idAttachment,
                name: anchor.textContent ?? '',
                isImage: anchor.title === AttachmentLinkConverter.imageTitle,
                host
            });
            this.store.acquire(idAttachment);
        }
        this.slots.set(slots);
        for (const slot of previous) {
            this.store.release(slot.idAttachment);
        }
        this.gallery?.register(
            this,
            slots
                .filter(slot => slot.isImage)
                .map(slot => ({ idAttachment: slot.idAttachment, name: slot.name }))
        );
    }

    private releaseSlots(): void {
        for (const slot of this.slots()) {
            this.store.release(slot.idAttachment);
        }
        this.slots.set([]);
    }

    private render(slot: AttachmentSlot, state: AttachmentState | null): void {
        if (!state || state.status === 'loading') {
            slot.host.replaceChildren(
                this.span(
                    'attachment-loading',
                    AttachmentRenderDirective.placeholderClass,
                    this.i18n.instant('ATTACHMENT.LOADING')
                )
            );
        } else if (state.status === 'missing') {
            slot.host.replaceChildren(
                this.span(
                    'attachment-missing',
                    AttachmentRenderDirective.placeholderClass,
                    this.i18n.instant('ATTACHMENT.MISSING'),
                    slot.name
                )
            );
        } else {
            slot.host.replaceChildren(
                slot.isImage ? this.thumb(slot, state) : this.card(slot, state)
            );
        }
    }

    private span(testid: string, className: string, text: string, title?: string): HTMLElement {
        const el = document.createElement('span');
        el.className = className;
        el.dataset['testid'] = testid;
        el.textContent = text;
        if (title) {
            el.title = title;
        }
        return el;
    }

    private thumb(
        slot: AttachmentSlot,
        state: Extract<AttachmentState, { status: 'ready' }>
    ): HTMLElement {
        const button = document.createElement('button');
        button.type = 'button';
        button.className = AttachmentRenderDirective.thumbClass;
        button.style.width = AttachmentRenderDirective.thumbWidth;
        button.style.height = AttachmentRenderDirective.thumbHeight;
        button.dataset['testid'] = 'attachment-thumb';
        button.dataset['idAttachment'] = slot.idAttachment;
        const img = document.createElement('img');
        img.src = state.url;
        img.alt = slot.name;
        img.className = 'block';
        img.style.width = '100%';
        img.style.height = '100%';
        img.style.objectFit = 'cover';
        button.append(img);
        button.addEventListener('click', () => this.gallery?.open(slot.idAttachment));
        return button;
    }

    private card(
        slot: AttachmentSlot,
        state: Extract<AttachmentState, { status: 'ready' }>
    ): HTMLElement {
        const card = document.createElement('a');
        card.className = AttachmentRenderDirective.cardClass;
        card.dataset['testid'] = 'attachment-card';
        card.href = state.url;
        if (state.mimeType === 'application/pdf') {
            card.target = '_blank';
            card.rel = 'noopener';
        } else {
            card.download = slot.name;
            card.title = this.i18n.instant('ATTACHMENT.DOWNLOAD');
        }
        const name = this.span('attachment-card-name', 'truncate', slot.name);
        const size = this.span(
            'attachment-card-size',
            'whitespace-nowrap ui-color-text-muted',
            AttachmentLinkConverter.toSizeLabel(state.size)
        );
        card.append(
            UiTablerSvg.create(
                state.mimeType === 'application/pdf' ? IconFileTypePdf : IconFile,
                20,
                'shrink-0'
            ),
            name,
            size
        );
        return card;
    }
}

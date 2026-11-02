import { DestroyRef, Directive, ElementRef, effect, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MarkdownComponent } from 'ngx-markdown';
import { UiHoverCard } from 'src/app/ui/service/ui-hover-card.service';
import { IssueLinkCardComponent } from '../components/issue-link-card/issue-link-card.component';
import { IssueLinkConverter } from '../converter/issue-link.converter';
import { IssueLinkRef } from '../model/issue-link.model';
import { IssueLinkInfoStore } from '../store/issue-link-info.store';

interface IssueLinkSlot {
    anchor: HTMLAnchorElement;
    ref: IssueLinkRef;
}

@Directive({
    selector: '[appIssueLinkRender]',
    standalone: false,
    host: {
        '(pointerover)': 'onPointerOver($event)',
        '(pointerout)': 'onPointerOut($event)',
        '(click)': 'onClick($event)'
    }
})
export class IssueLinkRenderDirective {
    private static readonly linkClass = 'issue-link';
    private static readonly finalClass = 'issue-link--final';

    private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
    private readonly store = inject(IssueLinkInfoStore);
    private readonly hoverCard = inject(UiHoverCard);
    private readonly slots = signal<IssueLinkSlot[]>([]);

    public constructor() {
        const destroyRef = inject(DestroyRef);
        destroyRef.onDestroy(() => this.hoverCard.closeWithin(this.host.nativeElement));
        inject(MarkdownComponent, { self: true })
            .ready.pipe(takeUntilDestroyed(destroyRef))
            .subscribe(() => this.collect());
        effect(() => {
            for (const slot of this.slots()) {
                const isFinal = this.store.view(slot.ref)?.isFinal ?? false;
                slot.anchor.classList.toggle(IssueLinkRenderDirective.finalClass, isFinal);
            }
        });
    }

    protected onPointerOver(event: PointerEvent): void {
        const slot = this.slotOf(event.target);
        if (!slot || event.pointerType !== 'mouse' || !this.store.view(slot.ref)) {
            return;
        }
        this.hoverCard.show(slot.anchor, IssueLinkCardComponent, ref =>
            ref.setInput('issueRef', slot.ref)
        );
    }

    protected onPointerOut(event: PointerEvent): void {
        const slot = this.slotOf(event.target);
        if (!slot || slot.anchor.contains(event.relatedTarget as Node | null)) {
            return;
        }
        this.hoverCard.hide(slot.anchor);
    }

    protected onClick(event: MouseEvent): void {
        const slot = this.slotOf(event.target);
        if (slot) {
            this.hoverCard.close();
        }
    }

    private collect(): void {
        const slots: IssueLinkSlot[] = [];
        for (const anchor of Array.from(
            this.host.nativeElement.querySelectorAll<HTMLAnchorElement>('a[href]')
        )) {
            const ref = IssueLinkConverter.toRef(anchor.getAttribute('href') ?? '');
            if (!ref || anchor.textContent !== `#${ref.idIssuePublic}`) {
                continue;
            }
            anchor.classList.add(IssueLinkRenderDirective.linkClass);
            anchor.dataset['testid'] = 'issue-link';
            slots.push({ anchor, ref });
        }
        this.slots.set(slots);
        this.store.request(slots.map(slot => slot.ref));
    }

    private slotOf(target: EventTarget | null): IssueLinkSlot | undefined {
        const anchor = (target as HTMLElement | null)?.closest?.('a');
        return anchor ? this.slots().find(slot => slot.anchor === anchor) : undefined;
    }
}

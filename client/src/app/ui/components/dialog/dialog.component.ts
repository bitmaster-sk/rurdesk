import { ConfigurableFocusTrap, ConfigurableFocusTrapFactory } from '@angular/cdk/a11y';
import { Overlay, OverlayRef } from '@angular/cdk/overlay';
import { TemplatePortal } from '@angular/cdk/portal';
import {
    ChangeDetectionStrategy,
    Component,
    OnDestroy,
    TemplateRef,
    ViewContainerRef,
    ViewEncapsulation,
    booleanAttribute,
    contentChild,
    effect,
    inject,
    input,
    model,
    output,
    untracked,
    viewChild
} from '@angular/core';

/** Deterministic id source for aria-labelledby (NOT Math.random — SSR/replay safe). */
let nextDialogId = 0;

/**
 * Centered modal. Content is projected into a body-level CDK overlay via a
 * `TemplatePortal` over `<ng-content>`-in-`<ng-template>` (same seam as
 * `ui-popover`), with a global centered position strategy + backdrop + block
 * scroll.
 *
 * Closing has three independent gates:
 *  - the `×` button (when `closable`),
 *  - Escape (when `closable() && closeOnEscape()`; the handler ignores
 *    `event.defaultPrevented` so a nested overlay, e.g. `ui-select`, that
 *    already handled Escape doesn't also close the dialog),
 *  - backdrop click (only when `dismissable()`, default `false`, so a stray
 *    click can't discard a form).
 *
 * a11y: `role=dialog` + `aria-modal`, `aria-labelledby` (header) or `aria-label`,
 * a CDK focus trap while open, and focus restoration to the trigger on close.
 */
@Component({
    selector: 'ui-dialog',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    template: `
        <ng-template #tpl>
            <div
                #panel
                class="ui-dialog"
                [class]="panelClass()"
                [style.width]="width()"
                [style.height]="height()"
                role="dialog"
                aria-modal="true"
                [attr.aria-labelledby]="header() ? titleId : null"
                [attr.aria-label]="!header() ? (ariaLabel() ?? null) : null"
            >
                @if (header() || closable()) {
                    <div class="ui-dialog__header">
                        <span class="ui-dialog__title" [id]="titleId">{{ header() }}</span>
                        @if (closable()) {
                            <button
                                type="button"
                                class="ui-dialog__close"
                                [attr.aria-label]="'SHARED.CLOSE' | translate"
                                (click)="close()"
                            >
                                <tabler-icon icon="x" [size]="18" />
                            </button>
                        }
                    </div>
                }
                <div class="ui-dialog__content" [class.ui-dialog__content--flush]="flush()">
                    <ng-content />
                </div>
                @if (footerTpl()) {
                    <div class="ui-dialog__footer">
                        <ng-container [ngTemplateOutlet]="footerTpl()!" />
                    </div>
                }
            </div>
        </ng-template>
    `
})
export class UiDialogComponent implements OnDestroy {
    /** Two-way open state; the overlay is attached/detached reactively from this. */
    public readonly visible = model<boolean>(false);

    public readonly header = input<string>();
    /** Accessible name when there is no `header` (else the dialog is just "dialog"). */
    public readonly ariaLabel = input<string>();
    /** Shows the `×` button. */
    public readonly closable = input(true, { transform: booleanAttribute });
    /** Escape closes — effective only together with `closable`. */
    public readonly closeOnEscape = input(true, { transform: booleanAttribute });
    /** Backdrop click closes. Default false, so a stray click can't discard a form. */
    public readonly dismissable = input(false, { transform: booleanAttribute });
    public readonly width = input<string>();
    public readonly height = input<string>();
    public readonly panelClass = input<string>('');
    /** Zero-padding, full-height content (mockup-card iframe). */
    public readonly flush = input(false, { transform: booleanAttribute });

    /** Fires on every open→closed transition (incl. programmatic close). */
    public readonly hide = output<void>();

    protected readonly titleId = `ui-dialog-title-${nextDialogId++}`;

    private readonly overlay = inject(Overlay);
    private readonly vcr = inject(ViewContainerRef);
    private readonly focusTrapFactory = inject(ConfigurableFocusTrapFactory);
    private readonly tpl = viewChild<TemplateRef<unknown>>('tpl');
    protected readonly footerTpl = contentChild<TemplateRef<unknown>>('footer');

    private overlayRef: OverlayRef | null = null;
    private focusTrap: ConfigurableFocusTrap | null = null;
    private previousActiveElement: HTMLElement | null = null;

    public constructor() {
        effect(() => {
            const open = this.visible();
            const tpl = this.tpl(); // both reads MUST be tracked, BEFORE untracked()
            if (open && !tpl) {
                // View not stamped yet (visible=true at creation) — the tpl() read
                // stays tracked, so this effect re-runs once the ref resolves.
                return;
            }
            untracked(() => {
                if (open) {
                    this.attach();
                } else {
                    this.detach();
                }
            });
        });
    }

    public ngOnDestroy(): void {
        this.teardown();
    }

    /** Request close: flips `visible`, which drives the effect + emits `hide`. */
    public close(): void {
        if (this.visible()) {
            this.visible.set(false);
        }
    }

    private attach(): void {
        if (this.overlayRef) {
            return;
        }
        this.previousActiveElement = document.activeElement as HTMLElement | null;

        this.overlayRef = this.overlay.create({
            positionStrategy: this.overlay
                .position()
                .global()
                .centerHorizontally()
                .centerVertically(),
            scrollStrategy: this.overlay.scrollStrategies.block(),
            hasBackdrop: true,
            backdropClass: 'ui-dialog__backdrop',
            // Split on whitespace: CDK adds each entry via classList.add, which
            // throws InvalidCharacterError on a space-containing string.
            panelClass: ['ui-dialog__pane', ...this.panelClass().split(/\s+/).filter(Boolean)]
        });

        this.overlayRef.attach(new TemplatePortal(this.tpl()!, this.vcr));

        this.focusTrap = this.focusTrapFactory.create(this.overlayRef.overlayElement);
        this.focusTrap.focusInitialElementWhenReady();

        this.overlayRef.backdropClick().subscribe(() => {
            if (this.dismissable()) {
                this.close();
            }
        });
        this.overlayRef.keydownEvents().subscribe(event => {
            // Ignore events a nested overlay already handled (e.g. ui-select's Esc),
            // otherwise the dialog would close alongside the inner panel.
            if (
                event.key === 'Escape' &&
                !event.defaultPrevented &&
                this.closable() &&
                this.closeOnEscape()
            ) {
                this.close();
            }
        });
    }

    private detach(): void {
        if (!this.overlayRef) {
            return;
        }
        this.teardown();
        this.hide.emit();
    }

    /** Dispose overlay + focus trap and restore focus, without emitting `hide`. */
    private teardown(): void {
        this.focusTrap?.destroy();
        this.focusTrap = null;
        this.overlayRef?.dispose();
        this.overlayRef = null;
        const prev = this.previousActiveElement;
        this.previousActiveElement = null;
        if (prev?.isConnected) {
            prev.focus();
        }
    }
}

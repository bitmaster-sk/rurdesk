import {
    ChangeDetectionStrategy,
    Component,
    computed,
    inject,
    input,
    output,
    signal
} from '@angular/core';
import { DomSanitizer, SafeHtml } from '@angular/platform-browser';

/**
 * Content Security Policy injected into every mockup document. Combined with the
 * iframe `sandbox="allow-scripts"` (no `allow-same-origin`), the mockup runs in
 * an opaque origin with no access to our app, its storage, or the API.
 */
const MOCKUP_CSP =
    "default-src 'none'; style-src 'unsafe-inline'; img-src data: https:; script-src 'unsafe-inline';";

/** Wrap raw mockup HTML in a minimal document with the locked-down CSP. */
function buildSrcdoc(html: string): string {
    return (
        '<!DOCTYPE html><html><head><meta charset="utf-8">' +
        `<meta http-equiv="Content-Security-Policy" content="${MOCKUP_CSP}">` +
        `</head><body>${html}</body></html>`
    );
}

/**
 * Compact card standing in for an HTML mockup in a message. Clicking it opens a
 * dialog that renders the mockup inside a sandboxed iframe via `[srcdoc]`. The
 * HTML is bypassed past Angular's sanitizer on purpose — the sandbox attribute,
 * not sanitization, is what isolates the mockup.
 */
@Component({
    selector: 'app-mockup-card',
    templateUrl: './mockup-card.component.html',
    styleUrls: ['./mockup-card.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class MockupCardComponent {
    private readonly sanitizer = inject(DomSanitizer);

    public readonly html = input.required<string>();
    public readonly title = input<string>();

    /** When true the card shows the "use & approve" action (agent run awaiting
     * approval, this message is the latest plan, and mockups are present). */
    public readonly approvable = input(false);
    /** This mockup is the approved winner — shows the ✓ badge, hides the action. */
    public readonly selected = input(false);
    /** Another mockup won — this one is greyed out as rejected. */
    public readonly rejected = input(false);

    public readonly useAndApprove = output<void>();

    protected readonly isOpen = signal(false);

    protected readonly displayTitle = computed(() => this.title() || '');

    protected readonly srcdoc = computed<SafeHtml>(() =>
        this.sanitizer.bypassSecurityTrustHtml(buildSrcdoc(this.html()))
    );

    protected onOpen(): void {
        this.isOpen.set(true);
    }

    protected onUse(event: Event): void {
        event.stopPropagation();
        this.useAndApprove.emit();
    }

    protected onVisibleChange(visible: boolean): void {
        this.isOpen.set(visible);
    }
}

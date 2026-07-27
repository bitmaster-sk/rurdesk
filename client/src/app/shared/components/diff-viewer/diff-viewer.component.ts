import {
    AfterViewInit,
    ChangeDetectionStrategy,
    Component,
    ElementRef,
    OnChanges,
    ViewEncapsulation,
    inject,
    input,
    viewChild
} from '@angular/core';
import { TranslateService } from '@ngx-translate/core';
import { html } from 'diff2html';
import { MrDiff, MrDiffFile } from 'src/app/project/model/git-integration.model';

// Tabler icons inlined as SVG so they render inside diff2html's
// innerHTML-rendered DOM (where the `<tabler-icon>` Angular component can't
// reach). Kept tiny — just the paths we need, no font dependency.
const ICON_EXTERNAL_LINK =
    '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" ' +
    'fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' +
    '<path d="M12 6h-6a2 2 0 0 0 -2 2v10a2 2 0 0 0 2 2h10a2 2 0 0 0 2 -2v-6"/>' +
    '<path d="M11 13l9 -9"/>' +
    '<path d="M15 4h5v5"/>' +
    '</svg>';
const ICON_LOADER =
    '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" ' +
    'fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' +
    '<path d="M12 3a9 9 0 1 0 9 9"/>' +
    '</svg>';
const ICON_CHEVRON =
    '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" ' +
    'fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' +
    '<path d="M6 9l6 6l6 -6"/>' +
    '</svg>';

export type DiffFileLinkBuilder = (file: MrDiffFile) => string | null;

/**
 * Renders any unified-diff patch using diff2html. Two input modes:
 *  - `diff`: a structured MrDiff (used by the issue MR/PR panel that fetches
 *    the patch from the git host API as a typed object).
 *  - `rawPatch`: a plain string already in unified-diff format (used by the
 *    plan-message renderer for ```diff fenced blocks the agent emits).
 *
 * Exactly one of the inputs should be set per use site. If both are set
 * `rawPatch` wins — it's the simpler shape and avoids the
 * re-header-rewriting logic the structured path needs.
 *
 * The component disables diff2html's built-in file-list header (it links to
 * an in-page anchor we don't expose) and instead enriches each per-file
 * header with an +added/-removed line summary and an optional external link
 * to the file on the source host. When the caller has a `MrDiff` it can pass
 * `fileLinkBuilder` to control where that icon points (e.g. github blob
 * URL). For raw-patch callers the host context is unknown so no link is
 * shown — only stats parsed out of the patch itself.
 */
@Component({
    selector: 'app-diff-viewer',
    templateUrl: './diff-viewer.component.html',
    styleUrls: ['./diff-viewer.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush,
    // diff2html injects the rendered diff via innerHTML, so its CSS classes
    // (`.d2h-*`) are not visible to Angular's view-encapsulation attribute
    // selectors. Turning encapsulation off lets the imported diff2html
    // stylesheet match the dynamic markup. The class names are well-namespaced
    // (`.d2h-*`, `.diff-viewer`) so the global leak is negligible.
    encapsulation: ViewEncapsulation.None
})
export class DiffViewerComponent implements AfterViewInit, OnChanges {
    private readonly i18n = inject(TranslateService);

    public readonly diff = input<MrDiff | null>(null);
    public readonly rawPatch = input<string | null>(null);
    public readonly fileLinkBuilder = input<DiffFileLinkBuilder | null>(null);
    // `loading` is for callers that fetch the diff asynchronously (e.g. the
    // issue MR/PR panel that hits the git host API on toggle). Toggled while
    // the request is in flight so the viewer renders a spinner instead of an
    // empty box. Plan-stage callers using `rawPatch` pass the data
    // synchronously and can leave this at the default false.
    public readonly loading = input(false);

    private readonly containerRef = viewChild<ElementRef<HTMLDivElement>>('diffContainer');

    public ngAfterViewInit(): void {
        this.render();
    }

    public ngOnChanges(): void {
        this.render();
    }

    private render(): void {
        const container = this.containerRef()?.nativeElement;
        if (!container) return;

        if (this.loading()) {
            container.innerHTML = `<div class="diff-viewer__loading"><span class="diff-viewer__spinner">${ICON_LOADER}</span>${this.i18n.instant('DIFF.LOADING')}</div>`;
            return;
        }

        const patch = this.resolvePatch();
        if (!patch) {
            container.innerHTML = '';
            return;
        }

        container.innerHTML = html(patch, {
            drawFileList: false,
            matching: 'lines',
            outputFormat: 'line-by-line'
        });

        this.enrichFileHeaders(container);
    }

    private resolvePatch(): string | null {
        const raw = this.rawPatch();
        if (raw) return raw;

        const diff = this.diff();
        if (!diff) return null;

        return diff.files.map(f => `--- a/${f.oldPath}\n+++ b/${f.newPath}\n${f.patch}`).join('\n');
    }

    /**
     * Appends a `+X / -Y` stat badge and (when a host link is available) an
     * external-link icon to each file header rendered by diff2html, and
     * wires the header itself as a click-to-collapse toggle for the file's
     * diff body. Relies on diff2html emitting `.d2h-file-wrapper` blocks in
     * the same order as `MrDiff.files`, so we can pair them positionally
     * without re-parsing file names back out of the DOM.
     *
     * The stats/link extras are injected inside `.d2h-file-name-wrapper`
     * (the existing flex row that holds the file name + tag) rather than
     * directly under `.d2h-file-header`. The name-wrapper already takes
     * width:100% and is a flex container with align-items:center —
     * appending here means the extras flow naturally on the right of the
     * name without forcing a wrap that would break diff2html's fixed-height
     * header and push the diff table down.
     *
     * Collapse uses diff2html's own `.d2h-d-none` class on `.d2h-file-diff`,
     * matching how diff2html itself toggles file visibility from the file
     * list — no extra display rules needed.
     */
    private enrichFileHeaders(container: HTMLElement): void {
        const wrappers = container.querySelectorAll<HTMLElement>('.d2h-file-wrapper');
        const files = this.resolveFiles();
        const builder = this.fileLinkBuilder();

        wrappers.forEach((wrapper, idx) => {
            const header = wrapper.querySelector<HTMLElement>('.d2h-file-header');
            const nameWrapper = wrapper.querySelector<HTMLElement>('.d2h-file-name-wrapper');
            const body = wrapper.querySelector<HTMLElement>('.d2h-file-diff');
            if (!header || !nameWrapper || !body) return;

            const file = files[idx];
            const patch = file?.patch ?? this.extractPatchFromHeader(wrapper);
            const { added, removed } = this.countChanges(patch);
            const url = file && builder ? builder(file) : null;

            this.prependChevron(nameWrapper);
            nameWrapper.appendChild(this.buildExtras(added, removed, url));
            this.wireCollapse(header, body);
        });
    }

    private prependChevron(nameWrapper: HTMLElement): void {
        const chevron = document.createElement('span');
        chevron.className = 'diff-viewer__chevron';
        chevron.innerHTML = ICON_CHEVRON;
        nameWrapper.prepend(chevron);
    }

    private wireCollapse(header: HTMLElement, body: HTMLElement): void {
        header.classList.add('diff-viewer__header--clickable');
        header.addEventListener('click', event => {
            // Let the host-link icon (and any future header buttons) keep
            // their own click semantics instead of also toggling collapse.
            const target = event.target as HTMLElement | null;
            if (target?.closest('.diff-viewer__file-link')) return;
            const collapsed = body.classList.toggle('d2h-d-none');
            header.classList.toggle('diff-viewer__header--collapsed', collapsed);
        });
    }

    private resolveFiles(): MrDiffFile[] {
        const diff = this.diff();
        if (diff) return diff.files;
        return [];
    }

    private buildExtras(added: number, removed: number, url: string | null): HTMLElement {
        const extras = document.createElement('span');
        extras.className = 'diff-viewer__file-extras';

        const stats = document.createElement('span');
        stats.className = 'diff-viewer__stats';
        stats.innerHTML =
            `<span class="diff-viewer__added">+${added}</span>` +
            `<span class="diff-viewer__removed">-${removed}</span>`;
        extras.appendChild(stats);

        if (url) {
            const a = document.createElement('a');
            a.className = 'diff-viewer__file-link';
            a.href = url;
            a.target = '_blank';
            a.rel = 'noopener noreferrer';
            a.title = this.i18n.instant('DIFF.OPEN_ON_HOST');
            a.innerHTML = ICON_EXTERNAL_LINK;
            extras.appendChild(a);
        }

        return extras;
    }

    /**
     * Fallback used when the viewer was given a `rawPatch` instead of a
     * structured `MrDiff` — we don't have per-file patch strings to count
     * against, so we read +/- lines out of what diff2html already rendered.
     */
    private extractPatchFromHeader(wrapper: HTMLElement): string {
        const lines: string[] = [];
        wrapper.querySelectorAll<HTMLElement>('.d2h-ins, .d2h-del').forEach(line => {
            const code = line.querySelector<HTMLElement>('.d2h-code-line-ctn');
            if (!code) return;
            lines.push((line.classList.contains('d2h-ins') ? '+' : '-') + (code.textContent ?? ''));
        });
        return lines.join('\n');
    }

    private countChanges(patch: string): { added: number; removed: number } {
        let added = 0;
        let removed = 0;
        for (const line of patch.split('\n')) {
            if (line.startsWith('+') && !line.startsWith('+++')) added++;
            else if (line.startsWith('-') && !line.startsWith('---')) removed++;
        }
        return { added, removed };
    }
}

import {
    AfterViewInit,
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    ElementRef,
    OnChanges,
    SimpleChanges,
    ViewEncapsulation,
    inject,
    input,
    viewChild
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { Diff2HtmlConfig, html } from 'diff2html';
import { FileContentLoader, MrDiff, MrDiffFile } from 'src/app/project/model/git-integration.model';
import { DiffParser, EXPAND_STEP, ExpandAction, ExpandDirection, ExpandSlot } from './diff-parser';

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
const ICON_FOLD_UP =
    '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" ' +
    'fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' +
    '<path d="M12 13v-8l-3 3m6 0l-3 -3"/>' +
    '<path d="M4 17h16"/>' +
    '</svg>';
const ICON_FOLD_DOWN =
    '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" ' +
    'fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' +
    '<path d="M12 11v8l3 -3m-6 0l3 3"/>' +
    '<path d="M4 7h16"/>' +
    '</svg>';
const ICON_FOLD =
    '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" ' +
    'fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' +
    '<path d="M12 15v6l3 -3m-6 0l3 3"/>' +
    '<path d="M12 9v-6l3 3m-6 0l3 -3"/>' +
    '<path d="M4 12h16"/>' +
    '</svg>';

const DIFF2HTML_CONFIG: Diff2HtmlConfig = {
    drawFileList: false,
    matching: 'lines',
    outputFormat: 'line-by-line'
};

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
    private readonly i18n = inject(I18nService);
    private readonly toast = inject(ToastNotificationService);
    private readonly destroyRef = inject(DestroyRef);

    public readonly diff = input<MrDiff | null>(null);
    public readonly rawPatch = input<string | null>(null);
    public readonly fileLinkBuilder = input<DiffFileLinkBuilder | null>(null);
    // `loading` is for callers that fetch the diff asynchronously (e.g. the
    // issue MR/PR panel that hits the git host API on toggle). Toggled while
    // the request is in flight so the viewer renders a spinner instead of an
    // empty box. Plan-stage callers using `rawPatch` pass the data
    // synchronously and can leave this at the default false.
    public readonly loading = input(false);
    public readonly fileContentLoader = input<FileContentLoader | null>(null);

    private readonly containerRef = viewChild<ElementRef<HTMLDivElement>>('diffContainer');

    private patches: string[] | null = null;
    private readonly fileLines = new Map<number, string[]>();
    private readonly pendingFiles = new Set<number>();

    public ngAfterViewInit(): void {
        this.render();
    }

    public ngOnChanges(changes: SimpleChanges): void {
        if (changes['diff']) {
            this.patches = this.diff()?.files.map(file => file.patch) ?? null;
            this.fileLines.clear();
            this.pendingFiles.clear();
        }
        this.render();
    }

    private render(): void {
        const container = this.containerRef()?.nativeElement;
        if (!container) return;

        if (this.loading()) {
            container.innerHTML = `<div class="diff-viewer__loading"><span class="diff-viewer__spinner">${ICON_LOADER}</span>${this.i18n.instant('DIFF.LOADING')}</div>`;
            return;
        }

        const raw = this.rawPatch();
        if (raw) {
            container.innerHTML = html(raw, DIFF2HTML_CONFIG);
            this.enrichFileHeaders(container);
            return;
        }

        const diff = this.diff();
        if (!diff) {
            container.innerHTML = '';
            return;
        }

        container.replaceChildren(
            ...diff.files.flatMap((_, idx) => this.renderFile(idx)?.parentElement ?? [])
        );
        container
            .querySelectorAll<HTMLElement>('.d2h-file-wrapper[data-idx]')
            .forEach(wrapper => this.enrichWrapper(wrapper, Number(wrapper.dataset['idx'])));
    }

    private renderFile(idx: number): HTMLElement | null {
        const scratch = document.createElement('div');
        scratch.innerHTML = html(this.filePatch(idx), DIFF2HTML_CONFIG);
        const wrapper = scratch.querySelector<HTMLElement>('.d2h-file-wrapper');
        wrapper?.setAttribute('data-idx', String(idx));
        return wrapper;
    }

    private findFileWrapper(idx: number): HTMLElement | null {
        return (
            this.containerRef()?.nativeElement.querySelector<HTMLElement>(
                `.d2h-file-wrapper[data-idx="${idx}"]`
            ) ?? null
        );
    }

    private filePatch(idx: number): string {
        const file = this.diff()?.files[idx];
        if (!file) return '';
        const patch = this.patches?.[idx] ?? file.patch;
        return `--- a/${file.oldPath}\n+++ b/${file.newPath}\n${this.fileModeLine(patch)}${patch}`;
    }

    private fileModeLine(patch: string): string {
        const [first] = DiffParser.parse(patch);
        if (first?.oldStart === 0 && first.oldLines === 0) return 'new file mode 100644\n';
        if (first?.newStart === 0 && first.newLines === 0) return 'deleted file mode 100644\n';
        return '';
    }

    /**
     * Appends a `+X / -Y` stat badge and (when a host link is available) an
     * external-link icon to each file header rendered by diff2html, and
     * wires the header itself as a click-to-collapse toggle for the file's
     * diff body. A `MrDiff` file is rendered on its own and its wrapper carries
     * `data-idx`, because diff2html drops files without hunks; a raw patch
     * has no file list, so its wrappers pair positionally.
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
        wrappers.forEach((wrapper, idx) => this.enrichWrapper(wrapper, idx));
    }

    private enrichWrapper(wrapper: HTMLElement, idx: number): void {
        const header = wrapper.querySelector<HTMLElement>('.d2h-file-header');
        const nameWrapper = wrapper.querySelector<HTMLElement>('.d2h-file-name-wrapper');
        const body = wrapper.querySelector<HTMLElement>('.d2h-file-diff');
        if (!header || !nameWrapper || !body) return;

        const file = this.resolveFiles()[idx];
        const builder = this.fileLinkBuilder();
        const patch = file?.patch ?? this.extractPatchFromHeader(wrapper);
        const { added, removed } = this.countChanges(patch);
        const url = file && builder ? builder(file) : null;

        this.prependChevron(nameWrapper);
        nameWrapper.appendChild(this.buildExtras(added, removed, url));
        this.wireCollapse(header, body);
        this.addExpanders(wrapper, idx);
    }

    private canExpand(): boolean {
        return !this.rawPatch() && !!this.fileContentLoader() && !!this.diff()?.headSha;
    }

    private addExpanders(wrapper: HTMLElement, idx: number): void {
        const patch = this.patches?.[idx];
        const tbody = wrapper.querySelector<HTMLElement>('.d2h-diff-tbody');
        if (!this.canExpand() || patch === undefined || !tbody) return;

        const hunks = DiffParser.parse(patch);
        const hunkHeaderCells = Array.from(
            tbody.querySelectorAll<HTMLElement>('td.d2h-code-linenumber.d2h-info')
        );
        if (hunkHeaderCells.length !== hunks.length) return;

        const slots = DiffParser.slots(hunks, this.fileLines.get(idx)?.length ?? null);
        for (const slot of slots) {
            const cell = hunkHeaderCells[slot.beforeHunk] ?? this.appendTrailingRow(tbody);
            cell.classList.add('diff-viewer__expanders');
            const isBetweenHunks = slot.beforeHunk > 0 && slot.beforeHunk < hunks.length;
            for (const action of slot.actions) {
                cell.appendChild(this.buildExpandButton(idx, slot, action, isBetweenHunks));
            }
        }
    }

    private appendTrailingRow(tbody: HTMLElement): HTMLElement {
        const row = document.createElement('tr');
        const numberCell = document.createElement('td');
        numberCell.className = 'd2h-code-linenumber d2h-info';
        const codeCell = document.createElement('td');
        codeCell.className = 'd2h-info';
        codeCell.innerHTML = '<div class="d2h-code-line"></div>';
        row.append(numberCell, codeCell);
        tbody.appendChild(row);
        return numberCell;
    }

    private buildExpandButton(
        idx: number,
        slot: ExpandSlot,
        action: ExpandAction,
        isBetweenHunks: boolean
    ): HTMLElement {
        const isAll = isBetweenHunks && slot.gap !== null && slot.gap <= EXPAND_STEP;
        const isUp = action.direction === ExpandDirection.Up;
        const button = document.createElement('button');
        button.type = 'button';
        button.className = 'diff-viewer__expand';
        button.innerHTML = isAll ? ICON_FOLD : isUp ? ICON_FOLD_UP : ICON_FOLD_DOWN;
        const label = this.i18n.instant(
            isAll ? 'DIFF.EXPAND_ALL' : isUp ? 'DIFF.EXPAND_UP' : 'DIFF.EXPAND_DOWN'
        );
        button.title = label;
        button.setAttribute('aria-label', label);
        button.addEventListener('click', () => this.onExpand(idx, action, button));
        return button;
    }

    private onExpand(idx: number, action: ExpandAction, button: HTMLButtonElement): void {
        if (this.pendingFiles.has(idx)) return;
        const cached = this.fileLines.get(idx);
        if (cached) {
            this.applyExpand(idx, cached, action);
            return;
        }

        const diff = this.diff();
        const loader = this.fileContentLoader();
        if (!diff || !loader) return;

        this.pendingFiles.add(idx);
        this.showLoading(idx, button);
        loader(diff.files[idx].newPath, diff.headSha)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: lines => {
                    if (this.diff() !== diff) return;
                    this.pendingFiles.delete(idx);
                    const normalized = lines.map(line => line.replace(/\r$/, ''));
                    this.fileLines.set(idx, normalized);
                    this.applyExpand(idx, normalized, action);
                },
                error: (error: { status?: number }) => {
                    if (this.diff() !== diff) return;
                    this.pendingFiles.delete(idx);
                    this.rerenderFile(idx);
                    this.toast.showError(
                        error.status === 409 ? 'DIFF.EXPAND_STALE' : 'DIFF.EXPAND_FAILED'
                    );
                }
            });
    }

    private showLoading(idx: number, button: HTMLButtonElement): void {
        const wrapper = this.findFileWrapper(idx);
        wrapper?.querySelectorAll<HTMLButtonElement>('.diff-viewer__expand').forEach(other => {
            other.disabled = true;
        });
        button.classList.add('diff-viewer__expand--loading');
        button.setAttribute('aria-busy', 'true');
        button.innerHTML = ICON_LOADER;
    }

    private applyExpand(idx: number, lines: string[], action: ExpandAction): void {
        if (!this.patches) return;
        const hunks = DiffParser.expand(DiffParser.parse(this.patches[idx]), lines, action);
        this.patches[idx] = DiffParser.serialize(hunks);
        this.rerenderFile(idx);
    }

    private rerenderFile(idx: number): void {
        const current = this.findFileWrapper(idx);
        const fresh = this.renderFile(idx);
        if (!current || !fresh) return;

        current.replaceWith(fresh);
        this.enrichWrapper(fresh, idx);
    }

    private prependChevron(nameWrapper: HTMLElement): void {
        const chevron = document.createElement('span');
        chevron.className = 'diff-viewer__chevron';
        chevron.innerHTML = ICON_CHEVRON;
        nameWrapper.prepend(chevron);
    }

    private wireCollapse(header: HTMLElement, body: HTMLElement): void {
        header.classList.add('diff-viewer__header--clickable');
        header.setAttribute('role', 'button');
        header.setAttribute('tabindex', '0');
        header.setAttribute('aria-expanded', 'true');
        header.setAttribute('aria-label', this.i18n.instant('DIFF.TOGGLE_FILE'));

        const toggle = (): void => {
            const collapsed = body.classList.toggle('d2h-d-none');
            header.classList.toggle('diff-viewer__header--collapsed', collapsed);
            header.setAttribute('aria-expanded', String(!collapsed));
        };

        header.addEventListener('click', event => {
            // Let the host-link icon (and any future header buttons) keep
            // their own click semantics instead of also toggling collapse.
            const target = event.target as HTMLElement | null;
            if (target?.closest('.diff-viewer__file-link')) return;
            toggle();
        });
        header.addEventListener('keydown', event => {
            if (event.key !== 'Enter' && event.key !== ' ') return;
            const target = event.target as HTMLElement | null;
            if (target?.closest('.diff-viewer__file-link')) return;
            event.preventDefault();
            toggle();
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

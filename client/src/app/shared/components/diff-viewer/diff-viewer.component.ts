import {
    AfterViewInit,
    ChangeDetectionStrategy,
    ChangeDetectorRef,
    Component,
    ElementRef,
    OnChanges,
    ViewEncapsulation,
    inject,
    input,
    signal,
    viewChild
} from '@angular/core';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { html } from 'diff2html';
import {
    DiffExpandDirection,
    FileContentLoader,
    MrDiff,
    MrDiffFile
} from 'src/app/project/model/git-integration.model';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { take } from 'rxjs/operators';
import { DiffExpander, ExpanderTarget } from './diff-expander';
import { DiffLineKind, DiffParser, ParsedDiffFile } from './diff-parser';

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
 * Renders unified diffs. Two input modes:
 *  - `diff`: structured MrDiff rendered with a custom Angular hunk renderer
 *    that supports incremental context expansion.
 *  - `rawPatch`: plain unified-diff string rendered with diff2html.
 * If both are set, `rawPatch` wins.
 */
@Component({
    selector: 'app-diff-viewer',
    templateUrl: './diff-viewer.component.html',
    styleUrls: ['./diff-viewer.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush,
    // diff2html injects rendered HTML for rawPatch mode, so its CSS classes
    // are not visible to Angular's view-encapsulation attribute selectors.
    // Disable encapsulation; the structured renderer uses the same global
    // class namespace.
    encapsulation: ViewEncapsulation.None
})
export class DiffViewerComponent implements AfterViewInit, OnChanges {
    protected readonly i18n = inject(I18nService);
    protected readonly toast = inject(ToastNotificationService);
    private readonly cdr = inject(ChangeDetectorRef);

    protected readonly iconLoader = ICON_LOADER;
    protected readonly iconChevron = ICON_CHEVRON;
    protected readonly iconExternalLink = ICON_EXTERNAL_LINK;

    public readonly diff = input<MrDiff | null>(null);
    public readonly rawPatch = input<string | null>(null);
    public readonly fileLinkBuilder = input<DiffFileLinkBuilder | null>(null);
    public readonly loading = input(false);
    public readonly fileContentLoader = input<FileContentLoader | null>(null);

    private readonly rawContainerRef = viewChild<ElementRef<HTMLDivElement>>('rawContainer');
    private readonly structuredContainerRef =
        viewChild<ElementRef<HTMLDivElement>>('structuredContainer');
    protected readonly parsedFiles = signal<ParsedDiffFile[]>([]);
    protected readonly collapsedFiles = signal<Set<number>>(new Set());
    private readonly fileContents = new Map<string, string[]>();
    private readonly fileLineCounts = new Map<string, number>();

    public ngAfterViewInit(): void {
        this.render();
    }

    public ngOnChanges(): void {
        this.render();
    }

    private render(): void {
        const rawContainer = this.rawContainerRef()?.nativeElement;
        const structuredContainer = this.structuredContainerRef()?.nativeElement;
        if (!rawContainer || !structuredContainer) return;

        if (this.loading()) {
            this.parsedFiles.set([]);
            rawContainer.innerHTML = '';
            this.cdr.markForCheck();
            return;
        }

        const raw = this.rawPatch();
        if (raw) {
            structuredContainer.innerHTML = '';
            this.parsedFiles.set([]);
            rawContainer.innerHTML = html(raw, {
                drawFileList: false,
                matching: 'lines',
                outputFormat: 'line-by-line'
            });
            this.enrichFileHeaders(rawContainer);
            this.cdr.markForCheck();
            return;
        }

        const diff = this.diff();
        if (!diff) {
            rawContainer.innerHTML = '';
            this.parsedFiles.set([]);
            return;
        }

        rawContainer.innerHTML = '';
        this.parsedFiles.set(
            diff.files.map(f => DiffParser.parseFile(f.oldPath, f.newPath, f.patch, f.isDeleted))
        );
        this.collapsedFiles.set(new Set());
        this.cdr.markForCheck();
    }

    protected refFor(isDeleted: boolean): string {
        const diff = this.diff();
        if (!diff) return '';
        return isDeleted ? diff.baseSha : diff.headSha;
    }

    protected pathFor(file: ParsedDiffFile): string {
        return file.newPath === '/dev/null' ? file.oldPath : file.newPath;
    }

    protected expandersFor(file: ParsedDiffFile, fileIdx: number): ExpanderTarget[] {
        return DiffExpander.list(file.hunks, this.fileLineCount(file, fileIdx), file.isDeleted);
    }

    private fileLineCount(file: ParsedDiffFile, fileIdx: number): number {
        const diff = this.diff();
        if (!diff) return 0;
        const key = this.contentCacheKey(file, this.refFor(diff.files[fileIdx].isDeleted));
        return this.fileLineCounts.get(key) ?? 0;
    }

    private contentCacheKey(file: ParsedDiffFile, ref: string): string {
        return `${this.pathFor(file)}:${ref}`;
    }

    protected onExpand(file: ParsedDiffFile, fileIdx: number, target: ExpanderTarget): void {
        const loader = this.fileContentLoader();
        if (!loader) return;
        const diff = this.diff();
        if (!diff) return;
        const isDeleted = diff.files[fileIdx].isDeleted;
        const ref = this.refFor(isDeleted);
        if (!ref) return;

        const hunk = file.hunks[target.hunkIndex];
        const count = DiffExpander.chunkSize(target.gap);
        const startLine = DiffExpander.startLine(hunk, target.direction, count, isDeleted);
        const key = this.contentCacheKey(file, ref);
        const cached = this.fileContents.get(key);
        if (cached) {
            this.applyExpansion(file, fileIdx, target, cached, startLine, count);
            return;
        }

        const req = {
            file: diff.files[fileIdx],
            ref,
            direction: target.direction,
            line: startLine,
            count
        };
        loader(req)
            .pipe(take(1))
            .subscribe({
                next: res => {
                    this.fileContents.set(key, res.lines);
                    this.fileLineCounts.set(key, res.lineCount);
                    this.applyExpansion(file, fileIdx, target, res.lines, startLine, count);
                },
                error: () => this.toast.showError('DIFF.EXPAND_FAILED')
            });
    }

    private applyExpansion(
        file: ParsedDiffFile,
        fileIdx: number,
        target: ExpanderTarget,
        allLines: string[],
        startLine: number,
        count: number
    ): void {
        const slice = allLines.slice(startLine, startLine + count);
        if (slice.length === 0) return;

        DiffParser.expandContext(file, target.hunkIndex, target.direction, slice);

        if (target.direction === DiffExpandDirection.Up && target.hunkIndex > 0) {
            DiffParser.mergeHunksIfAdjacent(file, target.hunkIndex - 1, target.hunkIndex);
        } else if (
            target.direction === DiffExpandDirection.Down &&
            target.hunkIndex < file.hunks.length - 1
        ) {
            DiffParser.mergeHunksIfAdjacent(file, target.hunkIndex, target.hunkIndex + 1);
        }

        this.parsedFiles.update(files => {
            const next = [...files];
            next[fileIdx] = { ...file, hunks: [...file.hunks] };
            return next;
        });
        this.cdr.markForCheck();
    }

    protected expanderIcon(target: ExpanderTarget): string {
        if (target.singleButton) return 'arrows-up-down';
        return target.direction === DiffExpandDirection.Up ? 'arrow-up' : 'arrow-down';
    }

    protected expanderLabel(target: ExpanderTarget): string {
        if (target.singleButton) {
            return this.i18n.instant('DIFF.EXPAND_ALL', { count: target.gap });
        }
        return this.i18n.instant('DIFF.EXPAND_N', { count: DiffExpander.chunkSize(target.gap) });
    }

    protected toggleFile(fileIdx: number): void {
        this.collapsedFiles.update(set => {
            const next = new Set(set);
            if (next.has(fileIdx)) next.delete(fileIdx);
            else next.add(fileIdx);
            return next;
        });
    }

    protected fileName(file: ParsedDiffFile): string {
        return file.newPath === '/dev/null' ? file.oldPath : file.newPath;
    }

    protected fileLinkFor(file: ParsedDiffFile): string | null {
        const diff = this.diff();
        const builder = this.fileLinkBuilder();
        if (!diff || !builder) return null;
        const idx = diff.files.findIndex(
            f => f.oldPath === file.oldPath && f.newPath === file.newPath
        );
        const mrFile = diff.files[idx];
        return mrFile ? builder(mrFile) : null;
    }

    protected statsFor(file: ParsedDiffFile): string {
        let added = 0;
        let removed = 0;
        for (const h of file.hunks) {
            for (const ln of h.lines) {
                if (ln.kind === DiffLineKind.Add) added++;
                else if (ln.kind === DiffLineKind.Remove) removed++;
            }
        }
        return `+${added} -${removed}`;
    }

    protected prefixFor(kind: DiffLineKind): string {
        switch (kind) {
            case DiffLineKind.Add:
                return '+';
            case DiffLineKind.Remove:
                return '-';
            default:
                return ' ';
        }
    }

    protected readonly DiffLineKind = DiffLineKind;
    protected readonly DiffExpandDirection = DiffExpandDirection;

    private enrichFileHeaders(container: HTMLElement): void {
        const wrappers = container.querySelectorAll<HTMLElement>('.d2h-file-wrapper');
        const files = this.diff()?.files ?? [];
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

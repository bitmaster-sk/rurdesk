import {
    AfterViewInit,
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    ElementRef,
    OnDestroy,
    computed,
    inject,
    input,
    output,
    signal,
    viewChild
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import {
    Completion,
    CompletionContext,
    CompletionResult,
    autocompletion,
    completionKeymap,
    startCompletion
} from '@codemirror/autocomplete';
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
import {
    HighlightStyle,
    Language,
    defineLanguageFacet,
    syntaxHighlighting
} from '@codemirror/language';
import { EditorState } from '@codemirror/state';
import { EditorView, keymap, placeholder } from '@codemirror/view';
import { tags } from '@lezer/highlight';
import { GFM, parser as markdownParser } from '@lezer/markdown';
import { finalize, take } from 'rxjs';
import { AttachmentScope } from 'src/app/shared/attachment/entity/attachment-scope.entity';
import { AttachmentUploader } from 'src/app/shared/attachment/service/attachment-uploader.service';
import { ClipboardContent } from 'src/app/shared/attachment/util/clipboard-content';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { UiMenuItem } from 'src/app/ui/components/menu/menu-item.model';
import { WikiCalloutKind } from '../../constants/wiki-callout-kind.enum';
import { WikiFormat } from '../../constants/wiki-format.enum';
import { WikiAnchorConverter } from '../../converter/wiki-anchor.converter';
import { WikiFormatConverter } from '../../converter/wiki-format.converter';
import { WikiFormatTool } from '../../entity/wiki-format-tool.entity';
import { WikiTableSize } from '../../entity/wiki-table-size.entity';
import { WikiTextEdit } from '../../entity/wiki-text-edit.entity';
import { WikiUploadPlaceholder } from '../../entity/wiki-upload-placeholder.entity';

export interface WikiEditorPageOption {
    title: string;
    slug: string;
    isShared: boolean;
}

export interface WikiEditorIssueOption {
    idIssuePublic: number;
    title: string;
}

@Component({
    selector: 'app-wiki-editor',
    templateUrl: './wiki-editor.component.html',
    styleUrls: ['./wiki-editor.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class WikiEditorComponent implements AfterViewInit, OnDestroy {
    public readonly initialValue = input<string>('');
    public readonly placeholderText = input<string>('');
    public readonly pages = input<WikiEditorPageOption[]>([]);
    public readonly issues = input<WikiEditorIssueOption[]>([]);
    public readonly attachmentScope = input<AttachmentScope | null>(null);

    public readonly valueChange = output<string>();
    public readonly issuesRequested = output<void>();

    private readonly uploader = inject(AttachmentUploader);
    private readonly i18n = inject(I18nService);
    private readonly destroyRef = inject(DestroyRef);
    private readonly pendingUploads = signal(0);
    private placeholders: WikiUploadPlaceholder[] = [];

    public readonly isUploading = computed(() => this.pendingUploads() > 0);

    protected readonly toolGroups: WikiFormatTool[][] = [
        [
            { format: WikiFormat.Heading1, icon: 'h-1', label: 'WIKI.FORMAT.HEADING1' },
            { format: WikiFormat.Heading2, icon: 'h-2', label: 'WIKI.FORMAT.HEADING2' },
            { format: WikiFormat.Heading3, icon: 'h-3', label: 'WIKI.FORMAT.HEADING3' }
        ],
        [
            { format: WikiFormat.Bold, icon: 'bold', label: 'WIKI.FORMAT.BOLD' },
            { format: WikiFormat.Italic, icon: 'italic', label: 'WIKI.FORMAT.ITALIC' },
            {
                format: WikiFormat.Strikethrough,
                icon: 'strikethrough',
                label: 'WIKI.FORMAT.STRIKETHROUGH'
            },
            { format: WikiFormat.Code, icon: 'code', label: 'WIKI.FORMAT.CODE' }
        ],
        [
            { format: WikiFormat.Link, icon: 'link', label: 'WIKI.FORMAT.LINK' },
            { format: WikiFormat.WikiLink, icon: 'brackets', label: 'WIKI.FORMAT.WIKI_LINK' }
        ],
        [
            { format: WikiFormat.BulletList, icon: 'list', label: 'WIKI.FORMAT.BULLET_LIST' },
            {
                format: WikiFormat.OrderedList,
                icon: 'list-numbers',
                label: 'WIKI.FORMAT.ORDERED_LIST'
            },
            { format: WikiFormat.TaskList, icon: 'list-check', label: 'WIKI.FORMAT.TASK_LIST' },
            { format: WikiFormat.Quote, icon: 'blockquote', label: 'WIKI.FORMAT.QUOTE' }
        ],
        [{ format: WikiFormat.CodeBlock, icon: 'source-code', label: 'WIKI.FORMAT.CODE_BLOCK' }]
    ];

    protected readonly calloutMenu: UiMenuItem[] = [
        {
            labelKey: 'WIKI.CALLOUT.NOTE',
            icon: 'info-circle',
            command: () => this.applyCallout(WikiCalloutKind.Note)
        },
        {
            labelKey: 'WIKI.CALLOUT.WARNING',
            icon: 'alert-triangle',
            command: () => this.applyCallout(WikiCalloutKind.Warning)
        },
        {
            labelKey: 'WIKI.CALLOUT.CAUTION',
            icon: 'alert-octagon',
            command: () => this.applyCallout(WikiCalloutKind.Caution)
        }
    ];

    private readonly highlightStyle = HighlightStyle.define([
        { tag: tags.heading, fontWeight: '700', color: 'var(--ui-color-primary-strong)' },
        { tag: tags.strong, fontWeight: '700' },
        { tag: tags.emphasis, fontStyle: 'italic' },
        { tag: tags.strikethrough, textDecoration: 'line-through' },
        { tag: [tags.link, tags.url], color: 'var(--ui-color-primary)' },
        { tag: tags.monospace, color: 'var(--ui-color-primary-strong)' },
        { tag: tags.quote, color: 'var(--ui-color-text-muted)' },
        {
            tag: [tags.processingInstruction, tags.contentSeparator, tags.labelName],
            color: 'var(--ui-color-text-muted)'
        }
    ]);

    private readonly host = viewChild.required<ElementRef<HTMLElement>>('host');
    private view: EditorView | null = null;
    private isApplyingExternalValue = false;

    public ngAfterViewInit(): void {
        const markdown = new Language(
            defineLanguageFacet(),
            markdownParser.configure(GFM),
            [],
            'markdown'
        );
        this.view = new EditorView({
            parent: this.host().nativeElement,
            state: EditorState.create({
                doc: this.initialValue(),
                extensions: [
                    history(),
                    markdown,
                    syntaxHighlighting(this.highlightStyle),
                    EditorView.lineWrapping,
                    placeholder(this.placeholderText()),
                    autocompletion({
                        override: [
                            (context: CompletionContext) => this.completePages(context),
                            (context: CompletionContext) => this.completeIssues(context)
                        ]
                    }),
                    keymap.of([
                        { key: 'Mod-b', run: () => this.applyFormat(WikiFormat.Bold) },
                        { key: 'Mod-i', run: () => this.applyFormat(WikiFormat.Italic) },
                        { key: 'Mod-k', run: () => this.applyFormat(WikiFormat.Link) },
                        ...completionKeymap,
                        ...defaultKeymap,
                        ...historyKeymap,
                        indentWithTab
                    ]),
                    EditorView.domEventHandlers({
                        paste: event => this.onPasteFiles(event),
                        drop: (event, view) => this.onDropFiles(event, view)
                    }),
                    EditorView.updateListener.of(update => {
                        if (!update.docChanged) {
                            return;
                        }
                        for (const placeholder of this.placeholders) {
                            placeholder.from = update.changes.mapPos(placeholder.from, 1);
                            placeholder.to = update.changes.mapPos(placeholder.to, -1);
                        }
                        if (!this.isApplyingExternalValue) {
                            this.valueChange.emit(update.state.doc.toString());
                        }
                    })
                ]
            })
        });
    }

    public ngOnDestroy(): void {
        this.view?.destroy();
        this.view = null;
    }

    public value(): string {
        return this.view?.state.doc.toString() ?? this.initialValue();
    }

    public replaceValue(text: string): void {
        if (!this.view) {
            return;
        }
        this.isApplyingExternalValue = true;
        this.view.dispatch({
            changes: { from: 0, to: this.view.state.doc.length, insert: text },
            selection: { anchor: Math.min(this.view.state.selection.main.head, text.length) }
        });
        this.isApplyingExternalValue = false;
    }

    public focus(): void {
        this.view?.focus();
    }

    protected onFilesPicked(fileInput: HTMLInputElement): void {
        const files = Array.from(fileInput.files ?? []);
        fileInput.value = '';
        this.uploadFiles(files);
    }

    protected onFormat(format: WikiFormat): void {
        this.applyFormat(format);
    }

    protected onTablePicked(size: WikiTableSize): void {
        this.applyEdit((doc, from, to) => WikiFormatConverter.toTableEdit(doc, from, to, size));
    }

    private onPasteFiles(event: ClipboardEvent): boolean {
        const files = Array.from(event.clipboardData?.files ?? []);
        if (
            !files.length ||
            ClipboardContent.isOfficeText(event.clipboardData) ||
            !this.attachmentScope()
        ) {
            return false;
        }
        event.preventDefault();
        this.uploadFiles(files);
        return true;
    }

    private onDropFiles(event: DragEvent, view: EditorView): boolean {
        const files = Array.from(event.dataTransfer?.files ?? []);
        if (!files.length || !this.attachmentScope()) {
            return false;
        }
        event.preventDefault();
        const position = view.posAtCoords({ x: event.clientX, y: event.clientY });
        if (position !== null) {
            view.dispatch({ selection: { anchor: position } });
        }
        this.uploadFiles(files);
        return true;
    }

    private uploadFiles(files: File[]): void {
        const scope = this.attachmentScope();
        const view = this.view;
        if (!scope || !view) {
            return;
        }
        for (const file of files) {
            const placeholder = this.insertPlaceholder(view, file.name);
            this.uploader
                .upload$(file, scope)
                .pipe(
                    take(1),
                    finalize(() => {
                        this.placeholders = this.placeholders.filter(
                            pending => pending !== placeholder
                        );
                        this.pendingUploads.update(count => count - 1);
                    }),
                    takeUntilDestroyed(this.destroyRef)
                )
                .subscribe({
                    next: ({ markdown }) => this.replacePlaceholder(placeholder, markdown),
                    error: () => this.replacePlaceholder(placeholder, '')
                });
        }
        view.focus();
    }

    private insertPlaceholder(view: EditorView, name: string): WikiUploadPlaceholder {
        const text = `[${this.i18n.instant('WIKI.ATTACHMENT.UPLOADING', { name })}]`;
        const { from, to } = view.state.selection.main;
        view.dispatch({
            changes: { from, to, insert: text },
            selection: { anchor: from + text.length },
            scrollIntoView: true,
            userEvent: 'input.paste'
        });
        const placeholder: WikiUploadPlaceholder = { from, to: from + text.length, text };
        this.placeholders.push(placeholder);
        this.pendingUploads.update(count => count + 1);
        return placeholder;
    }

    private replacePlaceholder(placeholder: WikiUploadPlaceholder, insert: string): void {
        const view = this.view;
        if (view?.state.sliceDoc(placeholder.from, placeholder.to) !== placeholder.text) {
            return;
        }
        view.dispatch({
            changes: { from: placeholder.from, to: placeholder.to, insert },
            userEvent: 'input.paste'
        });
    }

    private applyCallout(kind: WikiCalloutKind): void {
        this.applyEdit((doc, from, to) => WikiFormatConverter.toCalloutEdit(doc, from, to, kind));
    }

    private applyFormat(format: WikiFormat): boolean {
        const edit = this.applyEdit((doc, from, to) =>
            WikiFormatConverter.toEdit(doc, from, to, format)
        );
        if (
            this.view &&
            edit &&
            format === WikiFormat.WikiLink &&
            edit.selectionFrom === edit.selectionTo
        ) {
            startCompletion(this.view);
        }
        return edit !== null;
    }

    private applyEdit(
        build: (doc: string, from: number, to: number) => WikiTextEdit
    ): WikiTextEdit | null {
        const view = this.view;
        if (!view) {
            return null;
        }
        const { from, to } = view.state.selection.main;
        const edit = build(view.state.doc.toString(), from, to);
        view.dispatch({
            changes: { from: edit.from, to: edit.to, insert: edit.insert },
            selection: { anchor: edit.selectionFrom, head: edit.selectionTo },
            scrollIntoView: true,
            userEvent: 'input.format'
        });
        view.focus();
        return edit;
    }

    private closeLink(target: string): Completion['apply'] {
        return (view, _completion, from, to) => {
            const edit = WikiFormatConverter.toLinkCompletionEdit(
                view.state.doc.toString(),
                from,
                to,
                target
            );
            view.dispatch({
                changes: { from: edit.from, to: edit.to, insert: edit.insert },
                selection: { anchor: edit.selectionFrom }
            });
        };
    }

    private completePages(context: CompletionContext): CompletionResult | null {
        const match = context.matchBefore(/\[\[[^[\]\n]*/);
        if (!match) {
            return null;
        }
        if (match.text.startsWith('[[#')) {
            return {
                from: match.from + 3,
                options: WikiAnchorConverter.toHeadings(context.state.doc.toString()).map(
                    heading => ({ label: heading, apply: this.closeLink(heading), type: 'keyword' })
                ),
                validFor: /^[^[\]\n]*$/
            };
        }
        const options: Completion[] = this.pages().map(page => ({
            label: page.title,
            detail: page.isShared ? 'shared' : undefined,
            apply: this.closeLink(`${page.isShared ? 'shared:' : ''}${page.title}`),
            type: 'text'
        }));
        return { from: match.from + 2, options, validFor: /^[^[\]\n]*$/ };
    }

    private completeIssues(context: CompletionContext): CompletionResult | null {
        const match = context.matchBefore(/(^|[\s(])#\d*/);
        if (!match) {
            return null;
        }
        if (this.issues().length === 0) {
            this.issuesRequested.emit();
            return null;
        }
        const from = match.from + match.text.indexOf('#') + 1;
        const options: Completion[] = this.issues().map(issue => ({
            label: String(issue.idIssuePublic),
            detail: issue.title,
            type: 'variable'
        }));
        return { from, options, validFor: /^\d*$/ };
    }
}

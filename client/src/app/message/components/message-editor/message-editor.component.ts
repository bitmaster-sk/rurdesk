import {
    ChangeDetectionStrategy,
    Component,
    computed,
    DestroyRef,
    effect,
    ElementRef,
    forwardRef,
    inject,
    input,
    linkedSignal,
    model,
    output,
    signal,
    viewChild,
    AfterViewInit
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { User } from 'src/app/auth/model/user.model';
import { CodeBlockLanguage } from './constant/code-block-language.enum';
import { EditorCharacters } from './constant/editor-characters.enum';
import {
    applyMarker,
    atMentionBoundary,
    getLinearSelection,
    insertChipAtCaret,
    installPasteSanitizer,
    serialize,
    serializeRaw,
    setLinearSelection
} from './editor-text-model';
import { parseMentionParts, serializeMention } from 'src/app/shared/mention/mention-token.util';

type MessageChangeMode = 'onaction' | 'onchange' | 'onblur';

@Component({
    selector: 'app-message-editor',
    templateUrl: './message-editor.component.html',
    styleUrls: ['./message-editor.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => MessageEditorComponent),
            multi: true
        }
    ],
    standalone: false
})
export class MessageEditorComponent implements ControlValueAccessor, AfterViewInit {
    private readonly editorRef = viewChild.required<ElementRef<HTMLDivElement>>('editor');
    private readonly destroyRef = inject(DestroyRef);

    public readonly mode = input<'edit' | 'create'>('create');

    public readonly change = input<MessageChangeMode>('onaction');

    public readonly sendLabel = input<string | undefined>(undefined);

    public readonly sendIcon = input('send');

    public readonly disableSendButton = input(false);

    public readonly disableCancelButton = input(true);

    public readonly message = model('');

    public readonly mentionCandidates = input<User[]>([]);

    public readonly cancelled = output<void>();

    // Internal source of truth for the editor content. Derived from the model()
    // input via linkedSignal, so an inbound [message] push (or CVA writeValue,
    // which sets the model) recomputes it and re-hydrates the DOM — while local
    // writes (the user's own keystrokes via onInput) override it WITHOUT feeding
    // back into message(), so `messageChange` never fires per keystroke. The DOM
    // is hydrated from this via an effect (below), never synchronously (would
    // throw NG0951 pre view-init).
    protected readonly text = linkedSignal(() => this.message());

    protected readonly showCodeBlockSelector = signal(false);

    protected readonly CodeBlockLanguage: typeof CodeBlockLanguage = CodeBlockLanguage;

    // @mention autocomplete state
    protected readonly mentionQuery = signal<{ start: number; query: string } | null>(null);
    protected readonly activeIndex = signal(0);
    protected readonly filteredCandidates = computed(() => {
        const q = this.mentionQuery();
        if (!q) return [];
        const needle = q.query.toLowerCase();
        return this.mentionCandidates()
            .filter(u => u.name.toLowerCase().includes(needle))
            .slice(0, 8);
    });

    private _onChange: (value: string) => void = () => {};
    private _onTouch: (value: string) => void = () => {};

    // A DOM edit the component itself just performed. When set, the hydrate
    // effect skips this exact value so it never re-renders (and destroys the
    // caret) in response to our own onInput/toolbar/write.
    private lastRendered: string | null = null;

    public constructor() {
        // Hydrate the DOM from text(). Runs after view init (editorRef resolves),
        // so it never throws NG0951. Guarded so it does NOT re-render on the user's
        // own keystrokes / toolbar edits — only when the value genuinely differs
        // from what is currently in the editor.
        effect(() => {
            const body = this.text();
            const el = this.editorRef().nativeElement;
            if (serialize(el) === body) {
                this.lastRendered = body;
                return;
            }
            if (this.lastRendered === body) {
                return;
            }
            this.render(body);
            this.lastRendered = body;
        });
    }

    public onComponentFocusout(event: FocusEvent): void {
        const container = event.currentTarget as HTMLElement;
        if (!container.contains(event.relatedTarget as Node)) {
            if (this.change() === 'onblur') {
                this.propagateChange(this.text());
            }
        }
    }

    public onShiftEnter(evt: Event): void {
        evt.preventDefault();
        this.onSend();
    }

    protected onKeydown(evt: KeyboardEvent): void {
        const q = this.mentionQuery();
        const candidates = this.filteredCandidates();
        // Picker is only "active" when there is a query AND at least one candidate
        // visible. When zero candidates match, the picker is hidden and keys must
        // behave normally (Enter = newline, Shift+Enter = send).
        if (q && candidates.length > 0) {
            // Autocomplete is open — handle navigation keys.
            if (evt.key === 'ArrowDown') {
                evt.preventDefault();
                const max = candidates.length - 1;
                this.activeIndex.update(i => Math.min(i + 1, max));
                return;
            }
            if (evt.key === 'ArrowUp') {
                evt.preventDefault();
                this.activeIndex.update(i => Math.max(i - 1, 0));
                return;
            }
            if (evt.key === 'Enter') {
                evt.preventDefault();
                this.selectMention(candidates[this.activeIndex()]);
                return;
            }
            if (evt.key === 'Escape') {
                evt.preventDefault();
                this.mentionQuery.set(null);
                return;
            }
        }

        // Shift+Enter sends; plain Enter falls through so the browser inserts a
        // newline/<div>, which serialize() normalizes.
        if (evt.key === 'Enter' && evt.shiftKey) {
            this.onShiftEnter(evt);
        }
    }

    protected onInput(): void {
        const el = this.editorRef().nativeElement;
        const body = serialize(el);
        this.lastRendered = body; // our own edit — don't let the effect re-render
        this.text.set(body); // local override of the linkedSignal — does NOT feed
        // back into message(), so onaction/onblur consumers get no per-keystroke
        // messageChange. Only propagateChange (send/blur/onchange) emits outward.
        if (this.change() === 'onchange') {
            this.propagateChange(body);
        }
        this.detectMentionQuery(body);
    }

    private detectMentionQuery(body: string): void {
        const root = this.editorRef().nativeElement;
        const { start: caret } = getLinearSelection(root);
        const upto = body.slice(0, caret);
        // Extract the '@query' run at end of the text before the caret.
        const m = /@([^\s@]*)$/.exec(upto);
        if (!m) {
            this.mentionQuery.set(null);
            return;
        }
        // Gate on whether the '@' is at a valid boundary. The boundary is valid when
        // the character immediately before '@' is whitespace, BOF, or the '@' directly
        // follows a chip token in the serialized body (chip tokens end with ')').
        // A plain ')' in normal prose does NOT qualify — only chip-emitted ')' does,
        // and we distinguish these by inspecting the DOM via atMentionBoundary, which
        // walks the DOM's previous-sibling at the caret's position inside the '@query'
        // text node.
        const atIdx = caret - m[1].length - 1;
        // Temporarily move the caret to the '@' position so atMentionBoundary can
        // inspect the DOM previous-sibling at that exact point, then restore.
        setLinearSelection(root, atIdx, atIdx);
        const boundary = atMentionBoundary(root);
        setLinearSelection(root, caret, caret);
        if (boundary) {
            this.mentionQuery.set({ start: atIdx, query: m[1] });
            this.activeIndex.set(0);
        } else {
            this.mentionQuery.set(null);
        }
    }

    protected selectMention(user: User): void {
        const q = this.mentionQuery();
        if (!q) return;
        const root = this.editorRef().nativeElement;
        const { start: caret } = getLinearSelection(root);
        // Select the "@query" run (from the '@' to the current caret position),
        // then insert the chip via insertChipAtCaret. This lets deleteContents()
        // remove only the query text, then inserts the atomic chip span + trailing
        // space using Range.insertNode — which does NOT wipe the browser undo stack,
        // unlike replaceChildren/render() which does.
        setLinearSelection(root, q.start, caret);
        insertChipAtCaret(root, user.idUser, user.name);
        // Sync model from the updated DOM (no render() call). Inserting a chip is an
        // EDIT, not a send — mirror onInput: update local text and only propagate in
        // 'onchange' mode. Propagating unconditionally made consumers wired to
        // messageChange/onaction treat a mention pick as a send (auto-sent the chat
        // message / saved the comment).
        const next = serialize(root);
        this.lastRendered = next;
        this.text.set(next);
        if (this.change() === 'onchange') {
            this.propagateChange(next);
        }
        this.mentionQuery.set(null);
    }

    public onSend(): void {
        const value = this.text();
        if (!value || value.trim() === '') {
            return;
        }
        if (this.change() === 'onaction') {
            this.propagateChange(value);
            if (this.mode() === 'create') {
                this.text.set('');
            }
        }
    }

    private propagateChange(value: string): void {
        this.message.set(value);
        this._onChange(value);
        this._onTouch(value);
    }

    public focus(): void {
        this.editorRef().nativeElement.focus();
    }

    public onBold(): void {
        this.wrapSelection(EditorCharacters.BOLD, EditorCharacters.BOLD);
    }

    public onItalic(): void {
        this.wrapSelection(EditorCharacters.ITALIC, EditorCharacters.ITALIC);
    }

    public onStrikethrough(): void {
        this.wrapSelection(EditorCharacters.STRIKETHROUGH, EditorCharacters.STRIKETHROUGH);
    }

    public onOrderedList(): void {
        this.insertAtLineStart(EditorCharacters.ORDERED_LIST);
    }

    public onUnorderedList(): void {
        this.insertAtLineStart(EditorCharacters.UNORDERED_LIST);
    }

    public onCodeBlock(lang: CodeBlockLanguage): void {
        this.showCodeBlockSelector.set(false);
        const el = this.editorRef().nativeElement;
        el.focus();
        const sel = el.ownerDocument.getSelection();
        const selected = sel?.rangeCount ? sel.getRangeAt(0).toString() : '';
        const block =
            EditorCharacters.CODE_BLOCK +
            lang +
            '\n' +
            selected +
            '\n' +
            EditorCharacters.CODE_BLOCK;
        // execCommand insertText is undoable; Range.insertNode is not.
        el.ownerDocument.execCommand('insertText', false, block);
        this.onInput();
    }

    // Wrap the current selection in before/after markers (undoable via execCommand).
    private wrapSelection(before: string, after: string): void {
        const el = this.editorRef().nativeElement;
        el.focus();
        applyMarker(el, before, after);
        this.onInput();
    }

    // Insert a marker at the start of the caret's current line (undoable).
    private insertAtLineStart(marker: string): void {
        const el = this.editorRef().nativeElement;
        el.focus();
        // Untrimmed: getLinearSelection's caret (start) is in buildAtoms coordinates,
        // which include trailing empty lines. serialize() strips those, which would
        // shift the computed line start onto the last non-empty line when the caret
        // sits on a trailing empty line. serializeRaw shares the caret's coordinates.
        const body = serializeRaw(el);
        const { start } = getLinearSelection(el);
        const lineStart = body.lastIndexOf('\n', start - 1) + 1;
        // Move caret to line start, insert the marker, then restore caret past it.
        setLinearSelection(el, lineStart, lineStart);
        el.ownerDocument.execCommand('insertText', false, marker);
        this.onInput();
        const caret = start + marker.length;
        setLinearSelection(el, caret, caret);
    }

    // Render a serialized body string into the contenteditable as text + chips.
    private render(body: string): void {
        const root = this.editorRef().nativeElement;
        root.replaceChildren();
        for (const part of parseMentionParts(body)) {
            if (part.type === 'text') {
                if (part.text.length) {
                    root.appendChild(document.createTextNode(part.text));
                }
            } else {
                root.appendChild(this.buildChip(part.idUser, part.name));
            }
        }
    }

    private buildChip(idUser: number, name: string): HTMLElement {
        const span = document.createElement('span');
        span.className = 'mention-chip';
        span.contentEditable = 'false';
        span.dataset['token'] = serializeMention(idUser, name);
        span.dataset['id'] = String(idUser);
        span.textContent = '@' + name;
        return span;
    }

    public writeValue(value: string): void {
        // Never touches the DOM directly — the hydrate effect does that.
        this.text.set(value ?? '');
    }

    public registerOnChange(fn: (value: string) => void): void {
        this._onChange = fn;
    }

    public registerOnTouched(fn: (value: string) => void): void {
        this._onTouch = fn;
    }

    public ngAfterViewInit(): void {
        const cleanup = installPasteSanitizer(this.editorRef().nativeElement);
        this.destroyRef.onDestroy(cleanup);
    }
}

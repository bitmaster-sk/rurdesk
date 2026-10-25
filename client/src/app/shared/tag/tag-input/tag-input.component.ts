import {
    ChangeDetectionStrategy,
    Component,
    ElementRef,
    computed,
    effect,
    forwardRef,
    input,
    output,
    signal,
    untracked,
    viewChild
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { CreateTagReq, Tag } from 'src/app/tag/model/tag.model';
import { TAG_NAME_MAX_LENGTH } from 'src/app/tag/constants/tag.constants';

interface TagSuggestion {
    tag: Tag;
    isPrefix: boolean;
}

@Component({
    selector: 'app-tag-input',
    templateUrl: './tag-input.component.html',
    styleUrls: ['./tag-input.component.scss'],
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => TagInputComponent),
            multi: true
        }
    ],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class TagInputComponent implements ControlValueAccessor {
    private readonly maxSuggestions = 8;

    protected readonly nameMaxLength = TAG_NAME_MAX_LENGTH;

    private readonly queryInput = viewChild.required<ElementRef<HTMLInputElement>>('queryInput');

    public readonly idProject = input<number>();
    public readonly tags = input<Tag[]>([]);
    public readonly canCreate = input(false);
    public readonly colors = input.required<readonly string[]>();
    public readonly defaultColor = input.required<string>();
    public readonly inputId = input<string>();

    public readonly createTag = output<CreateTagReq>();

    protected readonly idsTag = signal<number[]>([]);
    protected readonly query = signal('');
    protected readonly isOpen = signal(false);
    protected readonly highlightedIndex = signal(0);
    protected readonly createColor = signal('');
    protected readonly isDisabled = signal(false);

    private onChange: (value: number[]) => void = () => {};
    private onTouched: () => void = () => {};

    protected readonly selectedTags = computed(() => {
        const byId = new Map(this.tags().map(tag => [tag.idTag, tag]));
        return this.idsTag()
            .map(idTag => byId.get(idTag))
            .filter((tag): tag is Tag => !!tag);
    });

    private readonly trimmedQuery = computed(() => this.query().trim());

    protected readonly suggestions = computed<TagSuggestion[]>(() => {
        const needle = this.trimmedQuery().toLowerCase();
        const selected = new Set(this.idsTag());
        return this.tags()
            .filter(tag => !selected.has(tag.idTag))
            .map(tag => ({ tag, position: tag.name.toLowerCase().indexOf(needle) }))
            .filter(match => match.position !== -1)
            .sort((a, b) => Number(a.position !== 0) - Number(b.position !== 0))
            .slice(0, this.maxSuggestions)
            .map(match => ({ tag: match.tag, isPrefix: match.position === 0 }));
    });

    private readonly isNewName = computed(() => {
        const name = this.trimmedQuery().toLowerCase();
        return name !== '' && !this.tags().some(tag => tag.name.toLowerCase() === name);
    });

    protected readonly isCreateOffered = computed(() => this.canCreate() && this.isNewName());

    protected readonly isCreateHighlighted = computed(
        () => this.isCreateOffered() && this.highlightedIndex() === this.suggestions().length
    );

    protected readonly isCreateForbidden = computed(() => !this.canCreate() && this.isNewName());

    protected readonly ghost = computed(() => {
        const first = this.suggestions()[0];
        if (!this.isOpen() || this.highlightedIndex() !== 0 || !first?.isPrefix) {
            return '';
        }
        return first.tag.name.slice(this.query().length);
    });

    protected readonly isPanelVisible = computed(
        () =>
            this.isOpen() &&
            this.trimmedQuery() !== '' &&
            (this.suggestions().length > 0 || this.isCreateOffered() || this.isCreateForbidden())
    );

    public constructor() {
        effect(() => {
            const color = this.defaultColor();
            untracked(() => this.createColor.set(color));
        });
    }

    protected onFieldClick(): void {
        this.queryInput().nativeElement.focus();
    }

    protected onQueryInput(value: string): void {
        this.query.set(value);
        this.highlightedIndex.set(0);
        this.createColor.set(this.defaultColor());
        this.isOpen.set(true);
    }

    protected onBlur(): void {
        this.isOpen.set(false);
        this.onTouched();
    }

    protected onKeydown(event: KeyboardEvent): void {
        switch (event.key) {
            case 'ArrowDown':
                this.moveHighlight(event, 1);
                break;
            case 'ArrowUp':
                this.moveHighlight(event, -1);
                break;
            case 'ArrowLeft':
            case 'ArrowRight':
                this.shiftCreateColor(event);
                break;
            case 'Tab':
            case 'Enter':
                this.acceptHighlighted(event);
                break;
            case 'Escape':
                if (this.isPanelVisible()) {
                    event.preventDefault();
                    event.stopPropagation();
                    this.isOpen.set(false);
                }
                break;
            case 'Backspace':
                if (this.query() === '' && this.idsTag().length > 0) {
                    this.writeIds(this.idsTag().slice(0, -1));
                }
                break;
        }
    }

    protected onPickSuggestion(tag: Tag): void {
        this.writeIds([...this.idsTag(), tag.idTag]);
        this.resetQuery();
    }

    protected onCreate(): void {
        const name = this.trimmedQuery();
        const idProject = this.idProject();
        if (!name || !idProject) {
            return;
        }
        this.createTag.emit({ idProject, name, color: this.createColor() });
        this.resetQuery();
    }

    protected onCreateColorChange(color: string): void {
        this.createColor.set(color);
    }

    protected onRemove(tag: Tag): void {
        this.writeIds(this.idsTag().filter(idTag => idTag !== tag.idTag));
    }

    public writeValue(value: number[] | null): void {
        this.idsTag.set(value ?? []);
    }

    public registerOnChange(fn: (value: number[]) => void): void {
        this.onChange = fn;
    }

    public registerOnTouched(fn: () => void): void {
        this.onTouched = fn;
    }

    public setDisabledState(isDisabled: boolean): void {
        this.isDisabled.set(isDisabled);
    }

    private moveHighlight(event: KeyboardEvent, step: number): void {
        const count = this.suggestions().length + (this.isCreateOffered() ? 1 : 0);
        if (!this.isPanelVisible() || count === 0) {
            return;
        }
        event.preventDefault();
        this.highlightedIndex.update(index => (index + step + count) % count);
    }

    private shiftCreateColor(event: KeyboardEvent): void {
        if (!this.isPanelVisible() || !this.isCreateHighlighted()) {
            return;
        }
        event.preventDefault();
        const colors = this.colors();
        const step = event.key === 'ArrowRight' ? 1 : -1;
        const index = Math.max(colors.indexOf(this.createColor()), 0);
        this.createColor.set(colors[(index + step + colors.length) % colors.length]);
    }

    private acceptHighlighted(event: KeyboardEvent): void {
        if (!this.isPanelVisible()) {
            return;
        }
        const suggestion = this.suggestions()[this.highlightedIndex()];
        if (suggestion) {
            event.preventDefault();
            this.onPickSuggestion(suggestion.tag);
            return;
        }
        if (this.isCreateHighlighted()) {
            event.preventDefault();
            this.onCreate();
        }
    }

    private writeIds(idsTag: number[]): void {
        this.idsTag.set(idsTag);
        this.onChange(idsTag);
    }

    private resetQuery(): void {
        this.query.set('');
        this.highlightedIndex.set(0);
        this.isOpen.set(false);
        this.queryInput().nativeElement.value = '';
    }
}

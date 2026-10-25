import {
    ChangeDetectionStrategy,
    Component,
    computed,
    forwardRef,
    input,
    signal
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { Tag } from 'src/app/tag/model/tag.model';

@Component({
    selector: 'app-tag-dropdown',
    templateUrl: './tag-dropdown.component.html',
    styleUrls: ['./tag-dropdown.component.scss'],
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => TagDropdownComponent),
            multi: true
        }
    ],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class TagDropdownComponent implements ControlValueAccessor {
    public readonly inputId = input<string>();
    public readonly tags = input<Tag[] | null>([]);

    protected readonly options = computed<Tag[]>(() => this.tags() ?? []);

    private readonly idsTag = signal<number[]>([]);

    protected readonly selectedTags = computed(() =>
        this.options().filter(tag => this.idsTag().includes(tag.idTag))
    );

    private onChange: (value: number[]) => void = () => {};
    private onTouched: () => void = () => {};

    protected onSelectionChange(tags: Tag[]): void {
        const idsTag = tags.map(tag => tag.idTag);
        this.idsTag.set(idsTag);
        this.onChange(idsTag);
        this.onTouched();
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
}

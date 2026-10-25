import { ChangeDetectionStrategy, Component, OnInit, inject, input, output } from '@angular/core';
import { FormControl, FormGroup, NonNullableFormBuilder, Validators } from '@angular/forms';
import { TAG_NAME_MAX_LENGTH } from '../../constants/tag.constants';
import { EditTagReq } from '../../model/tag.model';
import { TagColorService } from '../../service/tag-color.service';

interface TagForm {
    name: FormControl<string>;
    color: FormControl<string>;
}

@Component({
    selector: 'app-tag-form',
    templateUrl: './tag-form.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class TagFormComponent implements OnInit {
    protected readonly tagColor = inject(TagColorService);
    private readonly fb = inject(NonNullableFormBuilder);

    public readonly tag = input.required<Partial<EditTagReq>>();
    public readonly defaultColor = input.required<string>();

    public readonly save = output<EditTagReq>();
    public readonly cancelled = output<void>();

    protected form!: FormGroup<TagForm>;

    public ngOnInit(): void {
        this.form = this.fb.group<TagForm>({
            name: this.fb.control(this.tag().name ?? '', [
                Validators.required,
                Validators.maxLength(TAG_NAME_MAX_LENGTH)
            ]),
            color: this.fb.control(this.tag().color ?? this.defaultColor(), [Validators.required])
        });
    }

    protected onSave(): void {
        const source = this.tag();
        this.save.emit({
            idTag: source.idTag ?? 0,
            idProject: source.idProject ?? 0,
            name: this.form.controls.name.value.trim(),
            color: this.form.controls.color.value,
            orderRank: source.orderRank ?? 0
        });
    }

    protected onCancel(): void {
        this.cancelled.emit();
    }
}

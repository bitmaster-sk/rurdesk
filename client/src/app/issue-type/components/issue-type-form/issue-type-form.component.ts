import { ChangeDetectionStrategy, Component, OnInit, inject, input, output } from '@angular/core';
import { FormControl, FormGroup, NonNullableFormBuilder, Validators } from '@angular/forms';
import { IssueType } from '../../model/issue-type.model';

interface IssueTypeForm {
    name: FormControl<string>;
}

@Component({
    selector: 'app-issue-type-form',
    templateUrl: './issue-type-form.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class IssueTypeFormComponent implements OnInit {
    public readonly issueType = input.required<Partial<IssueType>>();

    public readonly save = output<IssueType>();

    public readonly cancelled = output<void>();

    protected form!: FormGroup<IssueTypeForm>;

    private readonly fb = inject(NonNullableFormBuilder);

    public ngOnInit(): void {
        this.form = this.fb.group<IssueTypeForm>({
            name: this.fb.control(this.issueType().name ?? '', [
                Validators.required,
                Validators.maxLength(20)
            ])
        });
    }

    protected onSave(): void {
        const source = this.issueType();
        this.save.emit({
            idIssueType: source.idIssueType ?? 0,
            idProject: source.idProject ?? 0,
            name: this.form.controls.name.value,
            protected: source.protected ?? false,
            orderRank: source.orderRank ?? 0
        });
    }

    protected onCancel(): void {
        this.cancelled.emit();
    }
}

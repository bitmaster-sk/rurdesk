import { Component, OnInit, inject, input, output } from '@angular/core';
import {
    FormBuilder,
    FormControl,
    FormGroup,
    NonNullableFormBuilder,
    Validators
} from '@angular/forms';
import { randomSeverityColor } from '../../constants/severity-colors';
import { IssueSeverity } from '../../model/issue-severity.model';

interface SeverityForm {
    idSeverity: FormControl<number | null>;
    idProject: FormControl<number | null>;
    title: FormControl<string>;
    color: FormControl<string>;
    orderRank: FormControl<number | null>;
}

@Component({
    selector: 'app-severity-form',
    templateUrl: './severity-form.component.html',
    styleUrls: ['./severity-form.component.scss'],
    standalone: false
})
export class SeverityFormComponent implements OnInit {
    public readonly severity = input.required<Partial<IssueSeverity>>();

    public readonly save = output<IssueSeverity>();

    public readonly cancelled = output<void>();

    public form!: FormGroup<SeverityForm>;

    private readonly fb = inject(FormBuilder);
    private readonly nfb = inject(NonNullableFormBuilder);

    public ngOnInit(): void {
        this.form = this.fb.group({
            idSeverity: this.fb.control<number | null>(this.severity().idSeverity ?? null),
            idProject: this.fb.control<number | null>(this.severity().idProject ?? null),
            title: this.nfb.control(this.severity().title ?? '', [
                Validators.required,
                Validators.maxLength(20)
            ]),
            // A new severity starts on a palette colour: an empty <input type="color">
            // renders black and blocks the required validator.
            color: this.nfb.control(this.severity().color ?? randomSeverityColor(), [
                Validators.required
            ]),
            orderRank: this.fb.control<number | null>(this.severity().orderRank ?? null)
        });
    }

    public onSave(): void {
        this.save.emit(this.form.getRawValue() as IssueSeverity);
    }

    public onCancel(): void {
        this.cancelled.emit();
    }
}

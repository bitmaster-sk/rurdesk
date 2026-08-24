import { Component, OnInit, inject, input, output } from '@angular/core';
import {
    FormBuilder,
    FormControl,
    FormGroup,
    NonNullableFormBuilder,
    Validators
} from '@angular/forms';
import { IssueState } from 'src/app/state/model/issue-state.model';

interface StateForm {
    idProject: FormControl<number | null>;
    idState: FormControl<number | null>;
    name: FormControl<string>;
    start: FormControl<boolean>;
    final: FormControl<boolean>;
    orderRank: FormControl<number | null>;
}

@Component({
    selector: 'app-state-form',
    templateUrl: './state-form.component.html',
    standalone: false
})
export class StateFormComponent implements OnInit {
    public readonly state = input.required<Partial<IssueState>>();

    public readonly save = output<IssueState>();

    public readonly cancelled = output<void>();

    public form!: FormGroup<StateForm>;

    private readonly fb = inject(FormBuilder);
    private readonly nfb = inject(NonNullableFormBuilder);

    public ngOnInit(): void {
        this.form = this.fb.group({
            idProject: this.fb.control<number | null>(this.state().idProject ?? null),
            idState: this.fb.control<number | null>(this.state().idState ?? null),
            name: this.nfb.control(this.state().name ?? '', [
                Validators.required,
                Validators.maxLength(20)
            ]),
            start: this.nfb.control(this.state().start ?? false),
            final: this.nfb.control(this.state().final ?? false),
            orderRank: this.fb.control<number | null>(this.state().orderRank ?? null)
        });
    }

    public onSave(): void {
        this.save.emit(this.form.getRawValue() as IssueState);
    }

    public onCancel(): void {
        this.cancelled.emit();
    }

    public get finalControl(): FormControl<boolean> {
        return this.form.controls.final;
    }

    public get startControl(): FormControl<boolean> {
        return this.form.controls.start;
    }
}

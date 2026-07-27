import { Component, EventEmitter, Input, OnInit, Output, inject } from '@angular/core';
import { FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
import { IssueState } from 'src/app/state/model/issue-state.model';

@Component({
    selector: 'app-state-form',
    templateUrl: './state-form.component.html',
    standalone: false
})
export class StateFormComponent implements OnInit {
    @Input() state: IssueState;

    @Output() save: EventEmitter<IssueState> = new EventEmitter<IssueState>();

    @Output() cancel: EventEmitter<void> = new EventEmitter<void>();

    public form: FormGroup = new FormGroup({});

    private fb = inject(FormBuilder);

    public ngOnInit(): void {
        this.form = this.fb.group({
            idProject: this.fb.control(this.state?.idProject),
            idState: this.fb.control(this.state?.idState),
            name: this.fb.control(this.state?.name, [
                Validators.required,
                Validators.maxLength(20)
            ]),
            start: this.fb.control(this.state?.start),
            final: this.fb.control(this.state?.final),
            orderRank: this.fb.control(this.state?.orderRank)
        });
    }

    public onSave(): void {
        this.save.emit(this.form.value);
    }

    public onCancel(): void {
        this.cancel.emit();
    }

    public get finalControl(): FormControl {
        return this.form.get('final') as FormControl;
    }

    public get startControl(): FormControl {
        return this.form.get('start') as FormControl;
    }
}

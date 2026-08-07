import { Component, OnInit, inject, input, output } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { randomSeverityColor } from '../../constants/severity-colors';
import { IssueSeverity } from '../../model/issue-severity.model';

@Component({
    selector: 'app-severity-form',
    templateUrl: './severity-form.component.html',
    styleUrls: ['./severity-form.component.scss'],
    standalone: false
})
export class SeverityFormComponent implements OnInit {
    public readonly severity = input.required<IssueSeverity>();

    public readonly save = output<IssueSeverity>();

    public readonly cancel = output<void>();

    public form: FormGroup = new FormGroup({});

    private fb = inject(FormBuilder);

    public ngOnInit(): void {
        this.form = this.fb.group({
            idSeverity: this.fb.control(this.severity().idSeverity),
            idProject: this.fb.control(this.severity().idProject),
            title: this.fb.control(this.severity().title, [
                Validators.required,
                Validators.maxLength(20)
            ]),
            // A new severity starts on a palette colour: an empty <input type="color">
            // renders black and blocks the required validator.
            color: this.fb.control(this.severity().color ?? randomSeverityColor(), [
                Validators.required
            ]),
            orderRank: this.fb.control(this.severity().orderRank)
        });
    }

    public onSave(): void {
        this.save.emit(this.form.value);
    }

    public onCancel(): void {
        this.cancel.emit();
    }
}

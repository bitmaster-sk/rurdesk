import { Component, EventEmitter, Input, OnInit, Output, inject } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { IssueSeverity } from '../../model/issue-severity.model';

@Component({
    selector: 'app-severity-form',
    templateUrl: './severity-form.component.html',
    styleUrls: ['./severity-form.component.scss'],
    standalone: false
})
export class SeverityFormComponent implements OnInit {
    @Input() severity: IssueSeverity;

    @Output() save: EventEmitter<IssueSeverity> = new EventEmitter<IssueSeverity>();

    @Output() cancel: EventEmitter<void> = new EventEmitter<void>();

    public form: FormGroup = new FormGroup({});

    private fb = inject(FormBuilder);

    public ngOnInit(): void {
        this.form = this.fb.group({
            idSeverity: this.fb.control(this.severity?.idSeverity),
            idProject: this.fb.control(this.severity?.idProject),
            title: this.fb.control(this.severity?.title, [
                Validators.required,
                Validators.maxLength(20)
            ]),
            color: this.fb.control(this.severity?.color, [Validators.required]),
            orderRank: this.fb.control(this.severity?.orderRank)
        });
    }

    public onSave(): void {
        this.save.emit(this.form.value);
    }

    public onCancel(): void {
        this.cancel.emit();
    }
}

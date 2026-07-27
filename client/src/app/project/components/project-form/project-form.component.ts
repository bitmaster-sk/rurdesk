import { Component, EventEmitter, inject, Input, OnDestroy, OnInit, Output } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { Subscription } from 'rxjs';
import { filter } from 'rxjs/operators';
import { UiSaveState } from '../../../ui/components/save-status/save-status-chip.component';
import { Project } from '../../model/project.model';

@Component({
    selector: 'app-project-form',
    templateUrl: './project-form.component.html',
    standalone: false
})
export class ProjectFormComponent implements OnInit, OnDestroy {
    @Input() saveOnBlur = false;

    /** Auto-save status shown as an inline chip on the name field (settings). */
    @Input() saveStatus: UiSaveState = UiSaveState.Idle;

    @Input() project: Project;

    @Output() save: EventEmitter<Project> = new EventEmitter<Project>();

    @Output() saveGenerate: EventEmitter<Project> = new EventEmitter<Project>();

    @Output() cancel: EventEmitter<void> = new EventEmitter<void>();

    public form: FormGroup = new FormGroup({});

    private subscription = new Subscription();

    private readonly fb = inject(FormBuilder);

    public ngOnInit(): void {
        this.form = this.fb.group({
            idProject: this.fb.control(this.project?.idProject),
            name: this.fb.control(this.project?.name, {
                validators: [Validators.required, Validators.maxLength(250)],
                updateOn: this.saveOnBlur ? 'blur' : 'change'
            })
        });

        if (this.saveOnBlur) {
            this.subscription.add(
                this.form.valueChanges
                    // Only auto-save a genuine change: a blur that didn't edit the name
                    // must not fire a redundant PUT (and flash the save chip).
                    .pipe(
                        filter(() => this.form.valid && this.form.value.name !== this.project?.name)
                    )
                    .subscribe(() => this.onSave())
            );
        }
    }

    public ngOnDestroy(): void {
        this.subscription.unsubscribe();
    }

    public onSave(): void {
        this.project.name = this.form.value.name;
        this.save.emit(this.project);
    }

    public onSaveGenerate(): void {
        this.project.name = this.form.value.name;
        this.saveGenerate.emit(this.project);
    }

    public onCancel(): void {
        this.cancel.emit();
    }
}

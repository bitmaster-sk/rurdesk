import { Component, inject, input, OnDestroy, OnInit, output } from '@angular/core';
import {
    FormBuilder,
    FormControl,
    FormGroup,
    NonNullableFormBuilder,
    Validators
} from '@angular/forms';
import { Subscription } from 'rxjs';
import { filter } from 'rxjs/operators';
import { UiSaveState } from '../../../ui/components/save-status/save-status-chip.component';
import { Project } from '../../model/project.model';

interface ProjectForm {
    idProject: FormControl<number | null>;
    name: FormControl<string>;
}

@Component({
    selector: 'app-project-form',
    templateUrl: './project-form.component.html',
    standalone: false
})
export class ProjectFormComponent implements OnInit, OnDestroy {
    public readonly saveOnBlur = input(false);

    /** Auto-save status shown as an inline chip on the name field (settings). */
    public readonly saveStatus = input<UiSaveState>(UiSaveState.Idle);

    public readonly project = input.required<Project>();

    public readonly save = output<Project>();

    public readonly saveGenerate = output<Project>();

    public readonly cancelled = output<void>();

    public form!: FormGroup<ProjectForm>;

    private subscription = new Subscription();

    private readonly fb = inject(FormBuilder);

    private readonly nfb = inject(NonNullableFormBuilder);

    public ngOnInit(): void {
        this.form = this.fb.group({
            idProject: this.fb.control<number | null>(this.project().idProject ?? null),
            name: this.nfb.control(this.project().name ?? '', {
                validators: [Validators.required, Validators.maxLength(250)],
                updateOn: this.saveOnBlur() ? 'blur' : 'change'
            })
        });

        if (this.saveOnBlur()) {
            this.subscription.add(
                this.form.valueChanges
                    // Only auto-save a genuine change: a blur that didn't edit the name
                    // must not fire a redundant PUT (and flash the save chip).
                    .pipe(
                        filter(
                            () =>
                                this.form.valid &&
                                this.form.controls.name.value !== this.project().name
                        )
                    )
                    .subscribe(() => this.onSave())
            );
        }
    }

    public ngOnDestroy(): void {
        this.subscription.unsubscribe();
    }

    public onSave(): void {
        this.save.emit(this.editedProject());
    }

    public onSaveGenerate(): void {
        this.saveGenerate.emit(this.editedProject());
    }

    public onCancel(): void {
        this.cancelled.emit();
    }

    private editedProject(): Project {
        return { ...this.project(), name: this.form.controls.name.value };
    }
}

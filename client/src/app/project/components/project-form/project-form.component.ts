import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    inject,
    input,
    OnInit,
    output
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import {
    FormBuilder,
    FormControl,
    FormGroup,
    NonNullableFormBuilder,
    Validators
} from '@angular/forms';
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
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ProjectFormComponent implements OnInit {
    public readonly saveOnBlur = input(false);

    /** Auto-save status shown as an inline chip on the name field (settings). */
    public readonly saveStatus = input<UiSaveState>(UiSaveState.Idle);

    public readonly project = input.required<Partial<Project>>();

    public readonly save = output<Project>();

    public readonly saveGenerate = output<Project>();

    public readonly cancelled = output<void>();

    public form!: FormGroup<ProjectForm>;

    private readonly fb = inject(FormBuilder);

    private readonly destroyRef = inject(DestroyRef);

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
            this.form.valueChanges
                // Only auto-save a genuine change: a blur that didn't edit the name
                // must not fire a redundant PUT (and flash the save chip).
                .pipe(
                    filter(
                        () =>
                            this.form.valid && this.form.controls.name.value !== this.project().name
                    ),
                    takeUntilDestroyed(this.destroyRef)
                )
                .subscribe(() => this.onSave());
        }
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
        const source = this.project();
        return {
            ...source,
            idProject: source.idProject ?? 0,
            color: source.color ?? '',
            name: this.form.controls.name.value
        };
    }
}

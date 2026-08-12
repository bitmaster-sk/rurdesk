import {
    ChangeDetectionStrategy,
    Component,
    computed,
    effect,
    inject,
    input,
    model,
    output
} from '@angular/core';
import { FormBuilder, Validators } from '@angular/forms';
import { Sprint } from '../../model/sprint.model';
import { addDays } from 'date-fns';
import { DateUtil } from 'src/app/shared/date/date.util';
import { DATE_ORDER_ERROR, dateOrder } from 'src/app/shared/validators/date-order.validator';

export interface SprintDialogSave {
    name: string;
    startAt: string;
    endAt: string;
}

const SPRINT_WINDOW_DAYS = 14;

@Component({
    selector: 'app-sprint-dialog',
    templateUrl: './sprint-dialog.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SprintDialogComponent {
    private readonly fb = inject(FormBuilder);

    public readonly visible = model<boolean>(false);
    /** The sprint being edited, or null to create a new one. */
    public readonly sprint = input<Sprint | null>(null);

    public readonly saved = output<SprintDialogSave>();
    public readonly deleted = output<void>();

    protected readonly isEdit = computed(() => !!this.sprint());

    protected readonly form = this.fb.group(
        {
            name: this.fb.control('', { validators: [Validators.maxLength(60)] }),
            startAt: this.fb.control<Date | null>(null, { validators: [Validators.required] }),
            endAt: this.fb.control<Date | null>(null, { validators: [Validators.required] })
        },
        { validators: [dateOrder('startAt', 'endAt')] }
    );

    constructor() {
        // Re-seed the form each time the dialog opens (create defaults or edit values).
        effect(() => {
            if (this.visible()) {
                this.seedForm(this.sprint());
            }
        });
    }

    protected saveHint(): string {
        if (this.form.valid) {
            return '';
        }
        if (this.form.errors?.[DATE_ORDER_ERROR]) {
            return 'ISSUE.KANBAN.SPRINTS.WINDOW_ORDER';
        }
        if (this.form.controls.name.invalid) {
            return this.form.controls.name.errors?.['maxlength']
                ? 'ISSUE.KANBAN.SPRINTS.NAME_TOO_LONG'
                : 'ISSUE.KANBAN.SPRINTS.NAME_REQUIRED';
        }
        return 'ISSUE.KANBAN.SPRINTS.DATES_REQUIRED';
    }

    protected onSubmit(): void {
        if (this.form.invalid) {
            this.form.markAllAsTouched();
            return;
        }
        const value = this.form.getRawValue();
        this.saved.emit({
            name: value.name?.trim() ?? '',
            startAt: DateUtil.toUtcDay(value.startAt as Date).toISOString(),
            endAt: DateUtil.toUtcDay(value.endAt as Date).toISOString()
        });
        this.visible.set(false);
    }

    protected onDelete(): void {
        this.deleted.emit();
        this.visible.set(false);
    }

    protected onCancel(): void {
        this.visible.set(false);
    }

    private seedForm(sprint: Sprint | null): void {
        // Name is required only when editing; on create an empty name lets the
        // backend auto-name ("Sprint N").
        this.form.controls.name.setValidators(
            sprint ? [Validators.required, Validators.maxLength(60)] : [Validators.maxLength(60)]
        );
        if (sprint) {
            this.form.reset({
                name: sprint.name,
                startAt: DateUtil.toLocalDay(sprint.startAt),
                endAt: DateUtil.toLocalDay(sprint.endAt)
            });
        } else {
            const start = new Date();
            const end = addDays(start, SPRINT_WINDOW_DAYS);
            this.form.reset({ name: '', startAt: start, endAt: end });
        }
        this.form.controls.name.updateValueAndValidity();
    }
}

import {
    ChangeDetectionStrategy,
    Component,
    effect,
    inject,
    input,
    model,
    output,
    signal
} from '@angular/core';
import { FormBuilder, Validators } from '@angular/forms';
import { AdminApi } from '../../api/admin.api.service';
import { Team } from '../../../team/model/team.model';

/**
 * TeamDialogComponent creates or edits a team (admin only).
 * Pass `team` for edit mode; leave null to create.
 */
@Component({
    selector: 'app-team-dialog',
    templateUrl: './team-dialog.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class TeamDialogComponent {
    private readonly fb = inject(FormBuilder);
    private readonly adminApi = inject(AdminApi);

    public readonly visible = model<boolean>(false);
    public readonly team = input<Team | null>(null);
    public readonly saved = output<Team>();

    protected readonly isSaving = signal(false);

    protected readonly form = this.fb.group({
        name: ['', [Validators.required, Validators.maxLength(250)]],
        color: ['#6b7280', [Validators.required]]
    });

    public constructor() {
        effect(() => {
            if (!this.visible()) return;
            const team = this.team();
            this.form.reset({
                name: team?.name ?? '',
                color: team?.color ?? '#6b7280'
            });
        });
    }

    protected onSubmit(): void {
        if (this.form.invalid) {
            this.form.markAllAsTouched();
            return;
        }
        const value = this.form.getRawValue();
        const team = this.team();
        const save$ = team
            ? this.adminApi.updateTeam$({
                  idTeam: team.idTeam,
                  name: value.name!,
                  color: value.color!
              })
            : this.adminApi.createTeam$({ name: value.name!, color: value.color! });

        this.isSaving.set(true);
        save$.subscribe({
            next: saved => {
                this.isSaving.set(false);
                this.visible.set(false);
                this.saved.emit(saved);
            },
            error: () => this.isSaving.set(false)
        });
    }

    protected onCancel(): void {
        this.visible.set(false);
    }
}

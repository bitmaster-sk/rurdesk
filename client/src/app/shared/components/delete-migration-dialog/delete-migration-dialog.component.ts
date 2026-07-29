import {
    ChangeDetectionStrategy,
    Component,
    computed,
    effect,
    input,
    model,
    output,
    signal
} from '@angular/core';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { IssueState } from 'src/app/state/model/issue-state.model';

/** Generic "delete X that is still in use" dialog — the host supplies labels and options. */
@Component({
    selector: 'app-delete-migration-dialog',
    templateUrl: './delete-migration-dialog.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class DeleteMigrationDialogComponent {
    public readonly visible = model<boolean>(false);
    public readonly entityLabel = input.required<string>();
    /** Pre-formatted usage lines ("3 tasks still use this state.") — rendered as a list. */
    public readonly usageItems = input<string[]>([]);
    /** Migration targets — exactly one of the two is non-empty per host (state vs severity). */
    public readonly stateOptions = input<IssueState[]>([]);
    public readonly severityOptions = input<IssueSeverity[]>([]);
    public readonly isLoading = input<boolean>(false);

    /** False = nothing references the row → plain confirm, no picker/radios. */
    public readonly hasUsage = input<boolean>(false);

    public readonly confirmed = output<{ migrateTo: number | null }>();

    public readonly mode = signal<'migrate' | 'unassign'>('migrate');
    public readonly selectedId = signal<number | null>(null);

    public readonly hasTargets = computed(
        () => this.stateOptions().length > 0 || this.severityOptions().length > 0
    );

    public readonly isConfirmDisabled = computed(
        () => this.hasUsage() && this.mode() === 'migrate' && this.selectedId() === null
    );

    public constructor() {
        effect(() => {
            if (this.visible()) {
                this.selectedId.set(null);
                this.mode.set(!this.hasUsage() || !this.hasTargets() ? 'unassign' : 'migrate');
            }
        });
    }

    public onConfirm(): void {
        if (this.isConfirmDisabled()) {
            return;
        }
        this.confirmed.emit({
            migrateTo: this.mode() === 'migrate' ? this.selectedId() : null
        });
    }

    public onCancel(): void {
        this.visible.set(false);
    }
}

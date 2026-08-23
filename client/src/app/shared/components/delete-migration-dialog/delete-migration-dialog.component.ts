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
import { DeleteMigrationOption, DeleteMigrationUsageItem } from './delete-migration-option.model';

/** Generic "delete X that is still in use" dialog — the host supplies labels and options. */
@Component({
    selector: 'app-delete-migration-dialog',
    templateUrl: './delete-migration-dialog.component.html',
    styleUrls: ['./delete-migration-dialog.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class DeleteMigrationDialogComponent {
    public readonly visible = model<boolean>(false);
    public readonly entityLabel = input.required<string>();
    /** Usage lines as translation keys — the dialog translates them in its template. */
    public readonly usageItems = input<DeleteMigrationUsageItem[]>([]);
    public readonly options = input<DeleteMigrationOption[]>([]);
    public readonly isLoading = input<boolean>(false);

    /** False = nothing references the row → plain confirm, no picker/radios. */
    public readonly hasUsage = input<boolean>(false);

    public readonly confirmed = output<{ migrateTo: number | null }>();

    public readonly mode = signal<'migrate' | 'unassign'>('migrate');
    public readonly selectedId = signal<number | null>(null);

    public readonly hasTargets = computed(() => this.options().length > 0);

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

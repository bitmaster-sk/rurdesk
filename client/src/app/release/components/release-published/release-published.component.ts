import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';
import { ReleaseTaskRow } from '../../entity/release-task-row.entity';
import { ReleaseView } from '../../model/release.model';

@Component({
    selector: 'app-release-published',
    templateUrl: './release-published.component.html',
    styleUrls: ['./release-published.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ReleasePublishedComponent {
    public readonly release = input.required<ReleaseView>();
    public readonly authorName = input('');
    public readonly canEdit = input(false);
    public readonly isEditing = input(false);
    public readonly isBusy = input(false);
    public readonly tasks = input<ReleaseTaskRow[] | null>(null);

    public readonly editStarted = output<void>();
    public readonly editCancelled = output<void>();
    public readonly notesSaved = output<string>();

    protected readonly notes = signal('');
    protected readonly canSave = computed(
        () => this.notes().trim() !== '' && this.notes() !== this.release().notes
    );

    protected onEdit(): void {
        this.notes.set(this.release().notes);
        this.editStarted.emit();
    }

    protected onNotesChange(notes: string): void {
        this.notes.set(notes);
    }

    protected onCancel(): void {
        this.editCancelled.emit();
    }

    protected onSave(): void {
        if (this.canSave()) {
            this.notesSaved.emit(this.notes());
        }
    }
}

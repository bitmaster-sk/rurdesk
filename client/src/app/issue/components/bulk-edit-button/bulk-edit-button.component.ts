import { ChangeDetectionStrategy, Component, inject, output } from '@angular/core';
import { IssueSelectionStore } from '../../store/issue-selection.store';

@Component({
    selector: 'app-bulk-edit-button',
    templateUrl: './bulk-edit-button.component.html',
    changeDetection: ChangeDetectionStrategy.OnPush,
    host: { style: 'display: contents' },
    standalone: false
})
export class BulkEditButtonComponent {
    protected readonly selection = inject(IssueSelectionStore);

    public readonly editRequested = output<void>();
}

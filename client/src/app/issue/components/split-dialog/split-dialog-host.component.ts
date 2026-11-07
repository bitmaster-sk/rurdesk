import { ChangeDetectionStrategy, Component, signal } from '@angular/core';
import { Issue } from '../../model/issue.model';

/**
 * Owns the open/close state around `app-split-dialog` so every issue view
 * (table, calendar, kanban, gantt, issue detail) hosts the split dialog with a
 * single tag instead of duplicating signals and handlers: the view calls
 * `open(issue)` on a template ref when split is requested, and the dialog
 * unmounts again after accepted or cancelled.
 */
@Component({
    selector: 'app-split-dialog-host',
    templateUrl: './split-dialog-host.component.html',
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class SplitDialogHostComponent {
    private readonly splitIssue = signal<Issue | null>(null);

    /** Opens the dialog for the given issue, replacing one already open. */
    public open(issue: Issue): void {
        this.splitIssue.set(issue);
    }

    private close(): void {
        this.splitIssue.set(null);
    }
}

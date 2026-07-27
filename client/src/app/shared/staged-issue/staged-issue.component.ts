import { ChangeDetectionStrategy, Component, input, linkedSignal, output } from '@angular/core';
import { StagedIssue } from './staged-issue.model';
import { IssueState } from '../../state/model/issue-state.model';
import { IssueSeverity } from '../../severity/model/issue-severity.model';
import { DurationParser } from '../duration/duration.parser';
import { DurationConverter } from '../duration/duration.converter';
import { DurationFormatter } from '../duration/duration.formatter';

@Component({
    selector: 'app-staged-issue',
    templateUrl: './staged-issue.component.html',
    styleUrls: ['./staged-issue.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class StagedIssueComponent {
    public issue = input.required<StagedIssue>();
    public states = input.required<IssueState[]>();
    public severities = input.required<IssueSeverity[]>();

    public issueChange = output<StagedIssue>();
    public delete = output<void>();

    protected editIssue = linkedSignal<StagedIssue>(() => this.issue());

    // Free-text duration ("1h 10m") for the estimate field. Recomputed only when
    // the issue identity (ref) changes — not on every estimatedMinutes update —
    // so typing isn't reformatted mid-edit (e.g. "90m" -> "1h 30m") under the caret.
    protected estimateText = linkedSignal({
        source: () => this.issue().ref,
        computation: () => this.minutesToText(this.issue().estimatedMinutes)
    });

    public onTitleChange(title: string): void {
        this.editIssue.update(i => ({ ...i, title }));
        this.issueChange.emit(this.editIssue());
    }

    public onDescriptionChange(description: string): void {
        this.editIssue.update(i => ({ ...i, description }));
        this.issueChange.emit(this.editIssue());
    }

    public onEstimateChange(value: string): void {
        this.estimateText.set(value);
        const seconds = DurationConverter.durationToSeconds(DurationParser.stringToDuration(value));
        const estimatedMinutes = Math.round(seconds / 60);
        this.editIssue.update(i => ({ ...i, estimatedMinutes }));
        this.issueChange.emit(this.editIssue());
    }

    public onSeverityChange(idSeverity: number | null): void {
        this.editIssue.update(i => ({ ...i, idSeverity }));
        this.issueChange.emit(this.editIssue());
    }

    public onStateChange(idState: number | null): void {
        this.editIssue.update(i => ({ ...i, idState }));
        this.issueChange.emit(this.editIssue());
    }

    public onDeleteClick(): void {
        this.delete.emit();
    }

    private minutesToText(minutes: number | null): string {
        if (!minutes) {
            return '';
        }
        return DurationFormatter.durationToString(
            DurationConverter.secondsToDuration(minutes * 60)
        );
    }
}

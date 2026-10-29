import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { ReleaseIssue } from '../../model/release.model';

@Component({
    selector: 'app-release-issue-list',
    templateUrl: './release-issue-list.component.html',
    styleUrls: ['./release-issue-list.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ReleaseIssueListComponent {
    public readonly idProject = input.required<number>();
    public readonly issues = input.required<ReleaseIssue[]>();
}

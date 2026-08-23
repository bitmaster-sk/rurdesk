import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { IssueType } from 'src/app/issue-type/model/issue-type.model';

@Component({
    selector: 'app-issue-type-badge',
    templateUrl: './issue-type-badge.component.html',
    styleUrls: ['./issue-type-badge.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class IssueTypeBadgeComponent {
    public readonly issueType = input<IssueType | undefined>(undefined);
    public readonly size = input<'xs' | 's' | 'default' | 'm' | 'l' | 'xl'>('default');

    protected readonly sizeClass = computed(() => `issue-type-badge--${this.size()}`);
}

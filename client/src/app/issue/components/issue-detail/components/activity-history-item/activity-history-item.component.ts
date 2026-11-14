import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { IssueHistoryField } from 'src/app/issue/constants/issue-history-field.enum';
import {
    HistoryRow,
    IssueHistoryGroup,
    IssueHistoryLookups
} from 'src/app/issue/entity/issue-history-group.entity';
import { IssueHistoryUser } from 'src/app/issue/model/issue-history.model';
import { HistoryGroupConverter } from '../issue-activity-feed/converter/history-group.converter';

@Component({
    selector: 'app-activity-history-item',
    templateUrl: './activity-history-item.component.html',
    styleUrls: ['./activity-history-item.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class ActivityHistoryItemComponent {
    public readonly group = input.required<IssueHistoryGroup>();
    public readonly user = input<IssueHistoryUser | undefined>(undefined);
    public readonly lookups = input.required<IssueHistoryLookups>();

    protected readonly isSystem = computed(() => this.group().source === 'system');

    protected readonly systemLabelKey = computed(() =>
        this.group().changes.some(change => change.field === IssueHistoryField.State)
            ? 'ISSUE.HISTORY.SYSTEM_GENERIC'
            : 'ISSUE.HISTORY.SYSTEM'
    );

    protected readonly rows = computed<HistoryRow[]>(() =>
        HistoryGroupConverter.toRows(this.group(), this.lookups())
    );
}

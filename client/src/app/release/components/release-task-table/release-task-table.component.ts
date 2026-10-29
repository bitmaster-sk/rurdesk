import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { ReleaseTaskRow } from '../../entity/release-task-row.entity';

@Component({
    selector: 'app-release-task-table',
    templateUrl: './release-task-table.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ReleaseTaskTableComponent {
    public readonly rows = input.required<ReleaseTaskRow[]>();
}

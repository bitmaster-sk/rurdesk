import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { Team } from 'src/app/team/model/team.model';

@Component({
    selector: 'app-team-label',
    templateUrl: './team-label.component.html',
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false,
    host: { class: 'inline-flex items-center gap-2 min-w-0' }
})
export class TeamLabelComponent {
    public readonly team = input.required<Pick<Team, 'name' | 'color'>>();
}

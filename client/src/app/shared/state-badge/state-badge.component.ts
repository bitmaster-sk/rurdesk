import { ChangeDetectionStrategy, Component, Input } from '@angular/core';
import { IssueState } from 'src/app/state/model/issue-state.model';

@Component({
    selector: 'app-state-badge',
    templateUrl: './state-badge.component.html',
    styleUrls: ['./state-badge.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class StateBadgeComponent {
    @Input() public state: IssueState | undefined;
    @Input() public size: 'xs' | 's' | 'default' | 'm' | 'l' | 'xl' = 'default';

    public get stateClass(): string {
        if (this.state?.start) return 'state-badge--start';
        if (this.state?.final) return 'state-badge--final';
        return 'state-badge--in-progress';
    }

    public get sizeClass(): string {
        return `state-badge--${this.size}`;
    }
}

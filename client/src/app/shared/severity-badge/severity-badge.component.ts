import { ChangeDetectionStrategy, Component, Input } from '@angular/core';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';

@Component({
    selector: 'app-severity-badge',
    templateUrl: './severity-badge.component.html',
    styleUrls: ['./severity-badge.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class SeverityBadgeComponent {
    @Input() public severity: IssueSeverity | undefined;
    @Input() public size: 'xs' | 's' | 'default' | 'm' | 'l' | 'xl' = 'default';

    public get sizeClass(): string {
        return `severity-badge--${this.size}`;
    }

    public get badgeColor(): string {
        return this.severity?.color ?? 'var(--ui-color-unknown)';
    }
}

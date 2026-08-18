import { Component, Input, OnChanges } from '@angular/core';
import { Color } from 'src/app/shared/color/color';

@Component({
    selector: 'app-avatar',
    templateUrl: './avatar.component.html',
    styleUrls: ['./avatar.component.scss'],
    standalone: false
})
export class AvatarComponent implements OnChanges {
    @Input() public height = 3;

    @Input() public width = 3;

    @Input() public radius = 1.5;

    @Input() public name = '';

    @Input() public bgColor = '';

    public initials = '';

    public textColor = '';

    public ngOnChanges(): void {
        if (this.name) {
            this.initials = this.buildInitials();
        }
        if (this.bgColor) {
            this.textColor = Color.getContrastColor(this.bgColor);
        }
    }

    private buildInitials(): string {
        const name = this.name.trim();
        const parts = name.split(' ');
        if (parts.length > 1) {
            return parts
                .slice(0, 2)
                .map(p => p[0])
                .join('')
                .toUpperCase();
        }
        return parts[0].substring(0, 2).toUpperCase();
    }
}

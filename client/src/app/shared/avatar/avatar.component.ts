import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { Color } from 'src/app/shared/color/color';

@Component({
    selector: 'app-avatar',
    templateUrl: './avatar.component.html',
    styleUrls: ['./avatar.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AvatarComponent {
    public readonly height = input(3);
    public readonly width = input(3);
    public readonly radius = input(1.5);
    public readonly name = input('');
    public readonly bgColor = input('');

    public readonly initials = computed(() => {
        const name = this.name().trim();
        if (!name) {
            return '';
        }
        const parts = name.split(' ');
        if (parts.length > 1) {
            return parts
                .slice(0, 2)
                .map(p => p[0])
                .join('')
                .toUpperCase();
        }
        return parts[0].substring(0, 2).toUpperCase();
    });

    public readonly textColor = computed(() => {
        const bgColor = this.bgColor();
        return bgColor ? Color.getContrastColor(bgColor) : '';
    });
}

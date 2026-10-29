import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { Color } from 'src/app/shared/color/color';
import { Initials } from './initials';

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
    public readonly manualInitials = input<string | null | undefined>(null);

    public readonly initials = computed(() => {
        const override = this.manualInitials()?.trim();
        if (override) {
            return override.toUpperCase();
        }
        return Initials.fromName(this.name());
    });

    public readonly textColor = computed(() => {
        const bgColor = this.bgColor();
        return bgColor ? Color.getContrastColor(bgColor) : '';
    });
}

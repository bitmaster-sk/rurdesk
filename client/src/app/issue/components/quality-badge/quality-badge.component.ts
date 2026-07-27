import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

@Component({
    selector: 'app-quality-badge',
    templateUrl: './quality-badge.component.html',
    styleUrls: ['./quality-badge.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class QualityBadgeComponent {
    public score = input<number | null | undefined>(null);

    public band = computed<'poor' | 'acceptable' | 'good' | null>(() => {
        const s = this.score();
        if (s == null) return null;
        if (s <= 40) return 'poor';
        if (s <= 70) return 'acceptable';
        return 'good';
    });
}

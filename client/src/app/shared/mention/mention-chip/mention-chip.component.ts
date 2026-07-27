import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { User } from 'src/app/auth/model/user.model';

@Component({
    selector: 'app-mention-chip',
    templateUrl: './mention-chip.component.html',
    styleUrls: ['./mention-chip.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class MentionChipComponent {
    public readonly idUser = input.required<number>();
    public readonly name = input('');
    public readonly candidates = input<Map<number, User> | User[] | null>(null);

    public readonly displayName = computed(() => {
        const c = this.candidates();
        const found =
            c instanceof Map ? c.get(this.idUser()) : c?.find(u => u.idUser === this.idUser());
        return found?.name ?? this.name();
    });
}

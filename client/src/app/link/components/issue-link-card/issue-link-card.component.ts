import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import { UiHoverCard } from 'src/app/ui/service/ui-hover-card.service';
import { IssueLinkRef } from '../../model/issue-link.model';
import { IssueLinkInfoStore } from '../../store/issue-link-info.store';

@Component({
    selector: 'app-issue-link-card',
    templateUrl: './issue-link-card.component.html',
    styleUrls: ['./issue-link-card.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class IssueLinkCardComponent {
    private readonly store = inject(IssueLinkInfoStore);
    private readonly hoverCard = inject(UiHoverCard);

    public readonly issueRef = input.required<IssueLinkRef>();

    protected readonly view = computed(() => this.store.view(this.issueRef()));

    protected onOpen(): void {
        this.hoverCard.close();
    }
}

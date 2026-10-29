import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { Tag } from 'src/app/tag/model/tag.model';
import { TagStore } from 'src/app/tag/store/tag.store';

@Component({
    selector: 'app-issue-tags',
    templateUrl: './issue-tags.component.html',
    styleUrls: ['./issue-tags.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class IssueTagsComponent {
    private readonly tagStore = inject(TagStore);

    public readonly idsTag = input<number[] | null | undefined>([]);
    public readonly max = input(3);

    private readonly tagsMap = toSignal(this.tagStore.tagsMap$, {
        initialValue: new Map<number, Tag>()
    });

    private readonly tags = computed(() => {
        const tagsMap = this.tagsMap();
        return (this.idsTag() ?? [])
            .map(idTag => tagsMap.get(idTag))
            .filter((tag): tag is Tag => !!tag);
    });

    protected readonly visibleTags = computed(() => this.tags().slice(0, this.max()));

    protected readonly hiddenTags = computed(() => this.tags().slice(this.max()));
}

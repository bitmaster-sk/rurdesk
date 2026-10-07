import {
    ChangeDetectionStrategy,
    Component,
    computed,
    effect,
    input,
    model,
    output,
    signal,
    untracked
} from '@angular/core';
import { WikiTreeConverter } from '../../converter/wiki-tree.converter';
import { WikiParentOption } from '../../entity/wiki-parent-option.entity';
import { WikiTreeNode } from '../../model/wiki-tree.model';

@Component({
    selector: 'app-wiki-move-dialog',
    templateUrl: './wiki-move-dialog.component.html',
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class WikiMoveDialogComponent {
    public readonly visible = model(false);
    public readonly nodes = input.required<WikiTreeNode[]>();
    public readonly idSpace = input.required<number>();
    public readonly idPage = input.required<number>();
    public readonly idParent = input<number | null>(null);
    public readonly rootLabel = input('');

    public readonly confirmed = output<number | null>();

    protected readonly selected = signal<number | null>(null);

    protected readonly options = computed<WikiParentOption[]>(() =>
        WikiTreeConverter.toParentOptions(
            this.nodes(),
            this.idSpace(),
            this.rootLabel(),
            WikiTreeConverter.subtreeIds(this.nodes(), this.idPage())
        )
    );

    public constructor() {
        effect(() => {
            if (this.visible()) {
                const idParent = this.idParent();
                untracked(() => this.selected.set(idParent));
            }
        });
    }

    protected onConfirm(): void {
        this.confirmed.emit(this.selected());
        this.visible.set(false);
    }
}

import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';
import { WikiMergeChunkKind } from '../../constants/wiki-merge-chunk-kind.enum';
import { WikiMergeConverter } from '../../converter/wiki-merge.converter';
import { WikiConflictChoice, WikiConflictPick } from '../../entity/wiki-conflict-choice.entity';
import { WikiMergeChunk } from '../../model/wiki-save.model';

@Component({
    selector: 'app-wiki-conflict',
    templateUrl: './wiki-conflict.component.html',
    styleUrls: ['./wiki-conflict.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class WikiConflictComponent {
    public readonly chunks = input.required<WikiMergeChunk[]>();
    public readonly theirsLabel = input<string>('');
    public readonly isSaving = input(false);

    public readonly resolved = output<string>();
    public readonly cancelled = output<void>();

    protected readonly Pick = WikiConflictPick;
    protected readonly ChunkKind = WikiMergeChunkKind;
    protected readonly choices = signal<ReadonlyMap<number, WikiConflictChoice>>(new Map());

    protected readonly conflictCount = computed(
        () => WikiMergeConverter.conflictIndexes(this.chunks()).length
    );
    protected readonly remaining = computed(
        () =>
            WikiMergeConverter.conflictIndexes(this.chunks()).filter(
                index => !this.choices().has(index)
            ).length
    );

    protected choice(index: number): WikiConflictChoice | undefined {
        return this.choices().get(index);
    }

    protected onPick(index: number, pick: WikiConflictPick): void {
        const chunk = this.chunks()[index];
        const custom =
            pick === WikiConflictPick.Custom
                ? [...(chunk.mine ?? []), ...(chunk.theirs ?? [])].join('\n')
                : '';
        this.setChoice(index, { pick, custom });
    }

    protected onCustomInput(index: number, event: Event): void {
        if (event.target instanceof HTMLTextAreaElement) {
            this.setChoice(index, { pick: WikiConflictPick.Custom, custom: event.target.value });
        }
    }

    protected onResolve(): void {
        if (!WikiMergeConverter.isResolved(this.chunks(), this.choices())) {
            return;
        }
        this.resolved.emit(WikiMergeConverter.toText(this.chunks(), this.choices()));
    }

    protected joined(lines: string[] | undefined): string {
        return (lines ?? []).join('\n');
    }

    private setChoice(index: number, choice: WikiConflictChoice): void {
        const next = new Map(this.choices());
        next.set(index, choice);
        this.choices.set(next);
    }
}

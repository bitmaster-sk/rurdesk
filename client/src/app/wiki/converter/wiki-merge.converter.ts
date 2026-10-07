import { WikiMergeChunkKind } from '../constants/wiki-merge-chunk-kind.enum';
import { WikiConflictChoice, WikiConflictPick } from '../entity/wiki-conflict-choice.entity';
import { WikiMergeChunk } from '../model/wiki-save.model';

export abstract class WikiMergeConverter {
    public static conflictIndexes(chunks: WikiMergeChunk[]): number[] {
        return chunks
            .map((chunk, index) => (chunk.kind === WikiMergeChunkKind.Conflict ? index : -1))
            .filter(index => index >= 0);
    }

    public static isResolved(
        chunks: WikiMergeChunk[],
        choices: ReadonlyMap<number, WikiConflictChoice>
    ): boolean {
        return WikiMergeConverter.conflictIndexes(chunks).every(index => choices.has(index));
    }

    public static toText(
        chunks: WikiMergeChunk[],
        choices: ReadonlyMap<number, WikiConflictChoice>
    ): string {
        const lines: string[] = [];
        chunks.forEach((chunk, index) => {
            if (chunk.kind === WikiMergeChunkKind.Stable) {
                lines.push(...(chunk.lines ?? []));
                return;
            }
            const choice = choices.get(index);
            switch (choice?.pick) {
                case WikiConflictPick.Theirs:
                    lines.push(...(chunk.theirs ?? []));
                    break;
                case WikiConflictPick.Both:
                    lines.push(...(chunk.mine ?? []), ...(chunk.theirs ?? []));
                    break;
                case WikiConflictPick.Custom:
                    lines.push(...choice.custom.split('\n'));
                    break;
                default:
                    lines.push(...(chunk.mine ?? []));
            }
        });
        return lines.join('\n');
    }
}

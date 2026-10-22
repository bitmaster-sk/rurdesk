import { ChangeDetectionStrategy, Component, input, signal } from '@angular/core';
import { RunWikiSection } from '../../constants/run-wiki-section.enum';
import { RunWikiStage } from '../../entity/run-wiki-context.entity';

@Component({
    selector: 'app-run-wiki-stage',
    templateUrl: './run-wiki-stage.component.html',
    styleUrls: ['./run-wiki-stage.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class RunWikiStageComponent {
    public readonly stage = input.required<RunWikiStage>();

    protected readonly RunWikiSection = RunWikiSection;
    protected readonly openSections = signal<ReadonlySet<RunWikiSection>>(
        new Set([RunWikiSection.Prompt])
    );

    protected isOpen(section: RunWikiSection): boolean {
        return this.openSections().has(section);
    }

    protected onToggle(section: RunWikiSection): void {
        this.openSections.update(open => {
            const next = new Set(open);
            if (!next.delete(section)) {
                next.add(section);
            }
            return next;
        });
    }

    protected pluralKey(key: string, count: number): string {
        return `${key}.${count === 1 ? 'SINGULAR' : 'PLURAL'}`;
    }
}

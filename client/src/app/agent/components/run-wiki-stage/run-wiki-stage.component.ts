import { ChangeDetectionStrategy, Component, input, signal } from '@angular/core';
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

    protected readonly isIndexOpen = signal(false);

    protected onToggleIndex(): void {
        this.isIndexOpen.update(isOpen => !isOpen);
    }
}

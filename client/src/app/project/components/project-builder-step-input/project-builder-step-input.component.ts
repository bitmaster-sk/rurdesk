import {
    ChangeDetectionStrategy,
    Component,
    computed,
    input,
    model,
    output,
    signal
} from '@angular/core';
import { IssueState } from '../../../state/model/issue-state.model';
import { IssueSeverity } from '../../../severity/model/issue-severity.model';

@Component({
    selector: 'app-project-builder-step-input',
    templateUrl: './project-builder-step-input.component.html',
    styleUrls: ['./project-builder-step-input.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class ProjectBuilderStepInputComponent {
    public description = model.required<string>();

    public defaultIdState = model<number | null>(null);

    public defaultIdSeverity = model<number | null>(null);

    public states = input.required<IssueState[]>();

    public severities = input.required<IssueSeverity[]>();

    public isGenerating = input.required<boolean>();

    public isGenerateDisabled = input.required<boolean>();

    public minChars = input<number>(10);

    public rateLimitCountdown = input.required<number>();

    public generate = output<void>();

    // Whether the user has edited the description yet (dirty, not touched — it
    // flips on the first change). A pristine field must not show the "too short"
    // error before the user has typed anything.
    private readonly isDirty = signal(false);

    // The description is too short to generate. Drives the inline hint + invalid
    // styling, but only once the field has been edited.
    public readonly isTooShort = computed(
        () => this.isDirty() && this.description().length < this.minChars()
    );

    public readonly remainingChars = computed(() => this.minChars() - this.description().length);

    public onDescriptionChange(value: string): void {
        this.isDirty.set(true);
        this.description.set(value);
    }

    public onGenerate(): void {
        this.generate.emit();
    }
}

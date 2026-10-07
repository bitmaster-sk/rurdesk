import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    OnInit,
    inject,
    input,
    signal
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormControl, Validators } from '@angular/forms';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';
import { WikiApi } from 'src/app/wiki/api/wiki.api.service';
import { Project } from '../../model/project.model';

@Component({
    selector: 'app-project-wiki-settings',
    templateUrl: './project-wiki-settings.component.html',
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class ProjectWikiSettingsComponent implements OnInit {
    private readonly api = inject(WikiApi);
    private readonly destroyRef = inject(DestroyRef);

    public readonly project = input.required<Project>();

    protected readonly tokenLimit = new FormControl<number>(0, {
        nonNullable: true,
        updateOn: 'blur',
        validators: [Validators.required, Validators.min(0), Validators.max(200000)]
    });
    protected readonly alwaysTokens = signal(0);
    protected readonly isLoaded = signal(false);
    protected readonly saveStatus = signal<UiSaveState>(UiSaveState.Idle);

    private savedLimit = 0;

    public ngOnInit(): void {
        this.api.loadTree$(this.project().idProject).subscribe(tree => {
            this.savedLimit = tree.tokenLimit;
            this.alwaysTokens.set(tree.alwaysTokens);
            this.tokenLimit.setValue(tree.tokenLimit, { emitEvent: false });
            this.isLoaded.set(true);
        });
        this.tokenLimit.valueChanges
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(() => this.onSave());
    }

    protected onSave(): void {
        const limit = Math.round(this.tokenLimit.value);
        if (this.tokenLimit.invalid || limit === this.savedLimit) {
            return;
        }
        this.saveStatus.set(UiSaveState.Saving);
        this.api.updateSettings$(this.project().idProject, limit).subscribe({
            next: () => {
                this.savedLimit = limit;
                this.saveStatus.set(UiSaveState.Saved);
            },
            error: () => this.saveStatus.set(UiSaveState.Error)
        });
    }
}

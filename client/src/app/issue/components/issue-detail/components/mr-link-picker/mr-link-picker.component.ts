import {
    ChangeDetectionStrategy,
    Component,
    OnInit,
    computed,
    inject,
    input,
    output,
    signal
} from '@angular/core';
import { FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
import { GitIntegrationRes } from 'src/app/project/model/git-integration.model';
import { GitIntegrationApi } from 'src/app/project/api/git-integration.api.service';
import { prMrLinkTitleKey } from 'src/app/issue/util/pr-mr-term';

@Component({
    selector: 'app-mr-link-picker',
    templateUrl: './mr-link-picker.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class MrLinkPickerComponent implements OnInit {
    public readonly idProject = input.required<number>();
    public readonly idGitIntegration = input<number | null | undefined>(null);
    public readonly mrId = input<string | null | undefined>(null);
    public readonly linked = output<{ idGitIntegration: number; mrId: string } | null>();
    public readonly cancelled = output<void>();

    private readonly fb = inject(FormBuilder);
    private readonly gitIntegrationApi = inject(GitIntegrationApi);

    protected readonly integrations = signal<GitIntegrationRes[]>([]);
    // selectedIntegrationId is mirrored from the form (which the template
    // patches when the dropdown changes) into a signal so the computed
    // title key recomputes on selection.
    protected readonly selectedIntegrationId = signal<number | null>(null);
    protected readonly titleKey = computed(() => {
        const id = this.selectedIntegrationId();
        const host =
            id == null
                ? null
                : (this.integrations().find(i => i.idGitIntegration === id)?.hostType ?? null);
        return prMrLinkTitleKey(host);
    });
    protected form!: FormGroup<{
        idGitIntegration: FormControl<number | null>;
        mrId: FormControl<string>;
    }>;

    public ngOnInit(): void {
        const initialId = this.idGitIntegration() ?? null;
        this.form = this.fb.group({
            idGitIntegration: this.fb.control<number | null>(initialId, Validators.required),
            mrId: this.fb.control(
                this.mrId() ?? '',
                [Validators.required, Validators.maxLength(50)]
            )
        }) as FormGroup<{
            idGitIntegration: FormControl<number | null>;
            mrId: FormControl<string>;
        }>;
        this.selectedIntegrationId.set(initialId);
        this.form
            .get('idGitIntegration')
            ?.valueChanges.subscribe(value =>
                this.selectedIntegrationId.set(typeof value === 'number' ? value : null)
            );

        this.gitIntegrationApi
            .list$(this.idProject())
            .subscribe(list => this.integrations.set(list));
    }

    protected get integrationOptions(): { label: string; value: number }[] {
        return this.integrations().map(i => ({ label: i.name, value: i.idGitIntegration }));
    }

    protected onSave(): void {
        if (this.form.invalid) return;
        const formValue = this.form.getRawValue();
        this.linked.emit({
            idGitIntegration: formValue.idGitIntegration!,
            mrId: formValue.mrId
        });
    }

    protected onUnlink(): void {
        this.linked.emit(null);
    }

    protected onCancel(): void {
        this.cancelled.emit();
    }
}

import {
    ChangeDetectionStrategy,
    Component,
    OnInit,
    inject,
    input,
    output,
    signal
} from '@angular/core';
import { FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
import {
    CreateGitIntegrationReq,
    GitIntegrationRes,
    HostType,
    UpdateGitIntegrationReq
} from '../../model/git-integration.model';
import { GitIntegrationApi } from '../../api/git-integration.api.service';
import { Project } from '../../model/project.model';
import { GIT_PROVIDERS } from '../../constants/git-provider.constants';

@Component({
    selector: 'app-git-integration-settings',
    templateUrl: './git-integration-settings.component.html',
    styleUrls: ['./git-integration-settings.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class GitIntegrationSettingsComponent implements OnInit {
    public readonly project = input.required<Project>();
    public readonly integration = input<GitIntegrationRes | null>(null);
    public readonly saved = output<GitIntegrationRes>();
    public readonly cancelled = output<void>();

    private readonly fb = inject(FormBuilder);
    private readonly gitIntegrationApi = inject(GitIntegrationApi);

    protected readonly isVisible = signal(true);
    protected readonly hostTypeOptions = GIT_PROVIDERS;

    protected form!: FormGroup<{
        name: FormControl<string>;
        hostType: FormControl<HostType>;
        baseUrl: FormControl<string>;
        repoPath: FormControl<string>;
        accessToken: FormControl<string>;
    }>;

    public ngOnInit(): void {
        const integration = this.integration();
        this.form = this.fb.group({
            name: this.fb.nonNullable.control(integration?.name ?? '', [
                Validators.required,
                Validators.maxLength(100)
            ]),
            hostType: this.fb.nonNullable.control(
                integration?.hostType ?? HostType.GitHub,
                Validators.required
            ),
            baseUrl: this.fb.nonNullable.control(integration?.baseUrl ?? '', [
                Validators.required,
                Validators.maxLength(255)
            ]),
            repoPath: this.fb.nonNullable.control(integration?.repoPath ?? '', [
                Validators.required,
                Validators.maxLength(255)
            ]),
            accessToken: this.fb.nonNullable.control('', integration ? [] : [Validators.required])
        });
    }

    protected get isEdit(): boolean {
        return !!this.integration();
    }

    protected onSave(): void {
        if (this.form.invalid) return;
        const integration = this.integration();
        if (integration) {
            const formValue = this.form.getRawValue();
            const req: UpdateGitIntegrationReq = {
                name: formValue.name,
                hostType: formValue.hostType,
                baseUrl: formValue.baseUrl,
                repoPath: formValue.repoPath
            };
            if (formValue.accessToken) {
                req.accessToken = formValue.accessToken;
            }
            this.gitIntegrationApi
                .update$(this.project().idProject, integration.idGitIntegration, req)
                .subscribe(result => this.saved.emit(result));
        } else {
            const formValue = this.form.getRawValue();
            const req: CreateGitIntegrationReq = {
                ...formValue
            };
            this.gitIntegrationApi
                .create$(this.project().idProject, req)
                .subscribe(result => this.saved.emit(result));
        }
    }

    protected onCancel(): void {
        this.cancelled.emit();
    }
}

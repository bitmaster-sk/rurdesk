import {
    ChangeDetectionStrategy,
    Component,
    OnInit,
    inject,
    input,
    output,
    signal
} from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
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

    protected form!: FormGroup;

    public ngOnInit(): void {
        const integration = this.integration();
        this.form = this.fb.group({
            name: [integration?.name ?? '', [Validators.required, Validators.maxLength(100)]],
            hostType: [integration?.hostType ?? HostType.GitHub, Validators.required],
            baseUrl: [integration?.baseUrl ?? '', [Validators.required, Validators.maxLength(255)]],
            repoPath: [
                integration?.repoPath ?? '',
                [Validators.required, Validators.maxLength(255)]
            ],
            accessToken: ['', integration ? [] : [Validators.required]]
        });
    }

    protected get isEdit(): boolean {
        return !!this.integration();
    }

    protected onSave(): void {
        if (this.form.invalid) return;
        const integration = this.integration();
        if (integration) {
            const req: UpdateGitIntegrationReq = {
                name: this.form.value.name,
                hostType: this.form.value.hostType,
                baseUrl: this.form.value.baseUrl,
                repoPath: this.form.value.repoPath
            };
            if (this.form.value.accessToken) {
                req.accessToken = this.form.value.accessToken;
            }
            this.gitIntegrationApi
                .update$(this.project().idProject, integration.idGitIntegration, req)
                .subscribe(result => this.saved.emit(result));
        } else {
            const req: CreateGitIntegrationReq = {
                name: this.form.value.name,
                hostType: this.form.value.hostType,
                baseUrl: this.form.value.baseUrl,
                repoPath: this.form.value.repoPath,
                accessToken: this.form.value.accessToken
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

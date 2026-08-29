import {
    ChangeDetectionStrategy,
    Component,
    OnInit,
    inject,
    input,
    output,
    signal
} from '@angular/core';
import { FormControl, FormGroup, NonNullableFormBuilder, Validators } from '@angular/forms';
import {
    CreateGitIntegrationReq,
    GitIntegrationRes,
    HostType,
    UpdateGitIntegrationReq
} from '../../model/git-integration.model';
import { GitIntegrationApi } from '../../api/git-integration.api.service';
import { Project } from '../../model/project.model';
import { GIT_PROVIDERS } from '../../constants/git-provider.constants';

interface GitIntegrationSettingsForm {
    name: FormControl<string>;
    hostType: FormControl<HostType>;
    baseUrl: FormControl<string>;
    repoPath: FormControl<string>;
    accessToken: FormControl<string>;
}

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

    private readonly fb = inject(NonNullableFormBuilder);
    private readonly gitIntegrationApi = inject(GitIntegrationApi);

    protected readonly isVisible = signal(true);
    protected readonly hostTypeOptions = GIT_PROVIDERS;

    protected form!: FormGroup<GitIntegrationSettingsForm>;

    public ngOnInit(): void {
        const integration = this.integration();
        this.form = this.fb.group<GitIntegrationSettingsForm>({
            name: this.fb.control(integration?.name ?? '', [
                Validators.required,
                Validators.maxLength(100)
            ]),
            hostType: this.fb.control(
                integration?.hostType ?? HostType.GitHub,
                Validators.required
            ),
            baseUrl: this.fb.control(integration?.baseUrl ?? '', [
                Validators.required,
                Validators.maxLength(255)
            ]),
            repoPath: this.fb.control(integration?.repoPath ?? '', [
                Validators.required,
                Validators.maxLength(255)
            ]),
            accessToken: this.fb.control('', integration ? [] : [Validators.required])
        });
    }

    protected get isEdit(): boolean {
        return !!this.integration();
    }

    protected onSave(): void {
        if (this.form.invalid) return;
        const integration = this.integration();
        const value = this.form.getRawValue();
        if (integration) {
            const req: UpdateGitIntegrationReq = {
                name: value.name,
                hostType: value.hostType,
                baseUrl: value.baseUrl,
                repoPath: value.repoPath
            };
            // Empty string = user didn't enter a new token → omit → backend keeps stored token.
            if (value.accessToken) {
                req.accessToken = value.accessToken;
            }
            this.gitIntegrationApi
                .update$(this.project().idProject, integration.idGitIntegration, req)
                .subscribe(result => this.saved.emit(result));
        } else {
            const req: CreateGitIntegrationReq = {
                name: value.name,
                hostType: value.hostType,
                baseUrl: value.baseUrl,
                repoPath: value.repoPath,
                accessToken: value.accessToken
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

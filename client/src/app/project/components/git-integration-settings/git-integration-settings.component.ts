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
import { FormControl, FormGroup, NonNullableFormBuilder, Validators } from '@angular/forms';
import isEqual from 'lodash-es/isEqual';
import { Observable, catchError, finalize, of, tap } from 'rxjs';
import {
    CheckGitIntegrationReq,
    CreateGitIntegrationReq,
    GitAccessCheckRes,
    GitAccessResult,
    GitIntegrationRes,
    HostType,
    UpdateGitIntegrationReq
} from '../../model/git-integration.model';
import { GitIntegrationApi } from '../../api/git-integration.api.service';
import { Project } from '../../model/project.model';
import { GIT_PROVIDERS, GIT_PROVIDER_BY_TYPE } from '../../constants/git-provider.constants';
import { UiMessageSeverity } from '../../../ui/components/message/message.component';

interface GitIntegrationSettingsForm {
    name: FormControl<string>;
    hostType: FormControl<HostType>;
    baseUrl: FormControl<string>;
    repoPath: FormControl<string>;
    baseBranch: FormControl<string>;
    accessToken: FormControl<string>;
}

const CHECK_SEVERITY: Record<GitAccessResult, UiMessageSeverity> = {
    [GitAccessResult.Ok]: 'success',
    [GitAccessResult.ReadOnly]: 'warn',
    [GitAccessResult.InvalidToken]: 'danger',
    [GitAccessResult.RepoNotFound]: 'danger',
    [GitAccessResult.Unreachable]: 'danger'
};

const CHECK_MESSAGE_KEY: Record<GitAccessResult, string> = {
    [GitAccessResult.Ok]: 'GIT_INTEGRATION.CHECK_RESULT.OK',
    [GitAccessResult.ReadOnly]: 'GIT_INTEGRATION.CHECK_RESULT.READ_ONLY',
    [GitAccessResult.InvalidToken]: 'GIT_INTEGRATION.CHECK_RESULT.INVALID_TOKEN',
    [GitAccessResult.RepoNotFound]: 'GIT_INTEGRATION.CHECK_RESULT.REPO_NOT_FOUND',
    [GitAccessResult.Unreachable]: 'GIT_INTEGRATION.CHECK_RESULT.UNREACHABLE'
};

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
    protected readonly isChecking = signal(false);
    protected readonly checkResult = signal<GitAccessCheckRes | null>(null);
    protected readonly checkMessage = computed(() => {
        const check = this.checkResult();
        if (!check) return null;
        return {
            severity: CHECK_SEVERITY[check.result],
            key: CHECK_MESSAGE_KEY[check.result],
            branch: check.defaultBranch ?? ''
        };
    });
    protected readonly isCheckFailed = computed(() => {
        const check = this.checkResult();
        return !!check && check.result !== GitAccessResult.Ok;
    });

    protected form!: FormGroup<GitIntegrationSettingsForm>;

    public ngOnInit(): void {
        const integration = this.integration();
        const hostType = integration?.hostType ?? HostType.GitHub;
        this.form = this.fb.group<GitIntegrationSettingsForm>({
            name: this.fb.control(integration?.name ?? '', [
                Validators.required,
                Validators.maxLength(100)
            ]),
            hostType: this.fb.control(hostType, Validators.required),
            baseUrl: this.fb.control(
                integration?.baseUrl ?? GIT_PROVIDER_BY_TYPE[hostType].defaultBaseUrl,
                [Validators.required, Validators.maxLength(255)]
            ),
            repoPath: this.fb.control(integration?.repoPath ?? '', [
                Validators.required,
                Validators.maxLength(255)
            ]),
            baseBranch: this.fb.control(integration?.baseBranch ?? '', Validators.maxLength(255)),
            accessToken: this.fb.control('', integration ? [] : [Validators.required])
        });
        this.form.controls.hostType.valueChanges.subscribe(selected =>
            this.prefillBaseUrl(selected)
        );
        this.form.valueChanges.subscribe(() => this.checkResult.set(null));
    }

    protected get isEdit(): boolean {
        return !!this.integration();
    }

    protected get canCheck(): boolean {
        const { hostType, baseUrl, repoPath, accessToken } = this.form.controls;
        return hostType.valid && baseUrl.valid && repoPath.valid && accessToken.valid;
    }

    protected onCheck(): void {
        if (!this.canCheck) return;
        this.check$().subscribe();
    }

    protected onSave(): void {
        if (this.form.invalid) return;
        if (this.checkResult()) {
            this.save();
            return;
        }
        this.check$().subscribe(() => {
            if (this.checkResult()?.result === GitAccessResult.Ok) this.save();
        });
    }

    protected onCancel(): void {
        this.cancelled.emit();
    }

    private prefillBaseUrl(hostType: HostType): void {
        const baseUrl = this.form.controls.baseUrl;
        const isProviderDefault = GIT_PROVIDERS.some(
            provider => provider.defaultBaseUrl === baseUrl.value
        );
        if (baseUrl.value === '' || isProviderDefault) {
            baseUrl.setValue(GIT_PROVIDER_BY_TYPE[hostType].defaultBaseUrl);
        }
    }

    private check$(): Observable<GitAccessCheckRes> {
        const value = this.form.getRawValue();
        const req: CheckGitIntegrationReq = {
            hostType: value.hostType,
            baseUrl: value.baseUrl,
            repoPath: value.repoPath
        };
        if (value.accessToken) {
            req.accessToken = value.accessToken;
        }
        const integration = this.integration();
        if (integration) {
            req.idGitIntegration = integration.idGitIntegration;
        }
        this.isChecking.set(true);
        return this.gitIntegrationApi.check$(this.project().idProject, req).pipe(
            catchError(() => of({ result: GitAccessResult.Unreachable })),
            tap(check => {
                if (isEqual(this.form.getRawValue(), value)) this.checkResult.set(check);
            }),
            finalize(() => this.isChecking.set(false))
        );
    }

    private save(): void {
        const integration = this.integration();
        const value = this.form.getRawValue();
        if (integration) {
            const req: UpdateGitIntegrationReq = {
                name: value.name,
                hostType: value.hostType,
                baseUrl: value.baseUrl,
                repoPath: value.repoPath,
                baseBranch: this.baseBranchOf(value.baseBranch)
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
                baseBranch: this.baseBranchOf(value.baseBranch),
                accessToken: value.accessToken
            };
            this.gitIntegrationApi
                .insert$(this.project().idProject, req)
                .subscribe(result => this.saved.emit(result));
        }
    }

    private baseBranchOf(value: string): string | null {
        return value.trim() || null;
    }
}

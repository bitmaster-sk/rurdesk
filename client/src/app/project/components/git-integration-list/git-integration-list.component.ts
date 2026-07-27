import { ChangeDetectionStrategy, Component, OnInit, inject, input, signal } from '@angular/core';
import { Project } from '../../model/project.model';
import { GitIntegrationRes, HostType } from '../../model/git-integration.model';
import { GitIntegrationApi } from '../../api/git-integration.api.service';
import { AclStore } from '../../store/acl.store';
import { GIT_PROVIDER_BY_TYPE } from '../../constants/git-provider.constants';

@Component({
    selector: 'app-git-integration-list',
    templateUrl: './git-integration-list.component.html',
    styleUrls: ['./git-integration-list.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class GitIntegrationListComponent implements OnInit {
    public readonly project = input.required<Project>();

    private readonly gitIntegrationApi = inject(GitIntegrationApi);
    protected readonly aclStore = inject(AclStore);

    protected readonly integrations = signal<GitIntegrationRes[]>([]);
    protected readonly isSettingsOpen = signal(false);
    protected readonly editTarget = signal<GitIntegrationRes | null>(null);

    public ngOnInit(): void {
        this.load();
    }

    protected providerIcon(hostType: HostType): string {
        return GIT_PROVIDER_BY_TYPE[hostType]?.icon ?? '';
    }

    protected providerLabel(hostType: HostType): string {
        return GIT_PROVIDER_BY_TYPE[hostType]?.label ?? hostType;
    }

    public onAdd(): void {
        this.editTarget.set(null);
        this.isSettingsOpen.set(true);
    }

    protected onEdit(integration: GitIntegrationRes): void {
        this.editTarget.set(integration);
        this.isSettingsOpen.set(true);
    }

    protected onDelete(integration: GitIntegrationRes): void {
        this.gitIntegrationApi
            .delete$(this.project().idProject, integration.idGitIntegration)
            .subscribe(() => {
                this.integrations.update(list =>
                    list.filter(i => i.idGitIntegration !== integration.idGitIntegration)
                );
            });
    }

    protected onSettingsSaved(saved: GitIntegrationRes): void {
        this.isSettingsOpen.set(false);
        const existing = this.integrations().findIndex(
            i => i.idGitIntegration === saved.idGitIntegration
        );
        if (existing >= 0) {
            this.integrations.update(list =>
                list.map(i => (i.idGitIntegration === saved.idGitIntegration ? saved : i))
            );
        } else {
            this.integrations.update(list => [...list, saved]);
        }
    }

    protected onSettingsCancelled(): void {
        this.isSettingsOpen.set(false);
    }

    private load(): void {
        this.gitIntegrationApi
            .list$(this.project().idProject)
            .subscribe(list => this.integrations.set(list));
    }
}

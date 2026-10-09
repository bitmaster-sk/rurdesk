import { ChangeDetectionStrategy, Component, inject, signal, WritableSignal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { ToastNotificationService } from '../../../core/toast-notification.service';
import { UiSaveState } from '../../../ui/components/save-status/save-status-chip.component';
import { Project } from '../../model/project.model';
import { ProjectStore } from '../../project.store';
import { AclStore } from '../../store/acl.store';

@Component({
    selector: 'app-project-settings',
    templateUrl: './project-settings.page.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ProjectSettingsPage {
    private readonly projectStore = inject(ProjectStore);
    private readonly toast = inject(ToastNotificationService);
    protected readonly aclStore = inject(AclStore);

    protected readonly project = toSignal(this.projectStore.project$);

    /** Auto-save status for the General panel, surfaced as an inline chip. */
    protected readonly generalSaveStatus = signal<UiSaveState>(UiSaveState.Idle);
    protected readonly stateSaveStatus = signal<UiSaveState>(UiSaveState.Idle);
    protected readonly severitySaveStatus = signal<UiSaveState>(UiSaveState.Idle);
    protected readonly issueTypeSaveStatus = signal<UiSaveState>(UiSaveState.Idle);

    protected onProjectSave(project: Project): void {
        this.persist(project, this.generalSaveStatus);
    }

    protected onStateSave(project: Project): void {
        this.persist(project, this.stateSaveStatus);
    }

    protected onSeveritySave(project: Project): void {
        this.persist(project, this.severitySaveStatus);
    }

    protected onIssueTypeSave(project: Project): void {
        this.persist(project, this.issueTypeSaveStatus);
    }

    protected onDefaultMigrated(project: Project): void {
        this.projectStore.setProject(project);
    }

    private persist(project: Project, status: WritableSignal<UiSaveState>): void {
        status.set(UiSaveState.Saving);
        this.projectStore.update(project).subscribe({
            next: () => status.set(UiSaveState.Saved),
            error: () => {
                status.set(UiSaveState.Error);
                this.toast.showError('PROJECT.SAVE_ERROR');
            }
        });
    }
}

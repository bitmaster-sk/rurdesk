import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { BehaviorSubject, of, throwError } from 'rxjs';
import { Component, Type, input, output, signal } from '@angular/core';
import { TranslateModule } from '@ngx-translate/core';
import { TablerIconStub } from 'src/testing/stubs';
import { Project } from '../../model/project.model';
import { ProjectStore } from '../../project.store';
import { AclStore } from '../../store/acl.store';
import { ToastNotificationService } from '../../../core/toast-notification.service';
import { UiSaveState } from '../../../ui/components/save-status/save-status-chip.component';
import { ProjectSettingsPage } from './project-settings.page';

@Component({ selector: 'app-project-form', template: '', standalone: true })
class ProjectFormStub {
    public readonly project = input<Project | null>(null);
    public readonly saveOnBlur = input<boolean>(false);
    public readonly saveStatus = input<UiSaveState>(UiSaveState.Idle);
    public readonly save = output<Project>();
}

@Component({ selector: 'app-project-state', template: '', standalone: true })
class ProjectStateStub {
    public readonly project = input<Project | null>(null);
    public readonly saveStatus = input<UiSaveState>(UiSaveState.Idle);
    public readonly save = output<Project>();
    public readonly defaultMigrated = output<Project>();
}

@Component({ selector: 'app-project-severity', template: '', standalone: true })
class ProjectSeverityStub {
    public readonly project = input<Project | null>(null);
    public readonly saveStatus = input<UiSaveState>(UiSaveState.Idle);
    public readonly save = output<Project>();
    public readonly defaultMigrated = output<Project>();
}

@Component({ selector: 'app-project-issue-type', template: '', standalone: true })
class ProjectIssueTypeStub {
    public readonly project = input<Project | null>(null);
    public readonly saveStatus = input<UiSaveState>(UiSaveState.Idle);
    public readonly save = output<Project>();
    public readonly defaultMigrated = output<Project>();
}

describe('ProjectSettingsPage (browser)', () => {
    const project: Project = {
        idProject: 10,
        name: 'P',
        color: '#000',
        idStateDefault: 2,
        idSeverityDefault: 3,
        idIssueTypeDefault: 1
    };

    let project$: BehaviorSubject<Project | null>;
    let update: ReturnType<typeof vi.fn>;
    let setProject: ReturnType<typeof vi.fn>;
    let showError: ReturnType<typeof vi.fn>;
    let fixture: ReturnType<typeof TestBed.createComponent<ProjectSettingsPage>>;

    beforeEach(async () => {
        project$ = new BehaviorSubject<Project | null>(project);
        update = vi.fn().mockReturnValue(of({ ...project }));
        setProject = vi.fn();
        showError = vi.fn();

        await TestBed.configureTestingModule({
            declarations: [ProjectSettingsPage],
            imports: [
                TranslateModule.forRoot(),
                TablerIconStub,
                ProjectFormStub,
                ProjectStateStub,
                ProjectSeverityStub,
                ProjectIssueTypeStub
            ],
            providers: [
                { provide: ProjectStore, useValue: { project$, update, setProject } },
                { provide: ToastNotificationService, useValue: { showError } },
                {
                    provide: AclStore,
                    useValue: {
                        canManageState: signal(true),
                        canManageSeverity: signal(true),
                        canManageIssueType: signal(true),
                        canManageCustomField: signal(false),
                        canReadMembers: signal(false),
                        canCreateMembers: signal(false),
                        canReadGitIntegration: signal(false),
                        canManageGitIntegration: signal(false)
                    }
                }
            ]
        }).compileComponents();

        fixture = TestBed.createComponent(ProjectSettingsPage);
        fixture.detectChanges();
    });

    function panel<T>(type: Type<T>): T {
        return fixture.debugElement.query(By.directive(type)).componentInstance as T;
    }

    it('routes a panel save through ProjectStore.update()', () => {
        const saved: Project = { ...project, idStateDefault: 5 };

        panel(ProjectStateStub).save.emit(saved);
        fixture.detectChanges();

        expect(update).toHaveBeenCalledTimes(1);
        expect(update).toHaveBeenCalledWith(saved);
        expect(setProject).not.toHaveBeenCalled();
        expect(panel(ProjectStateStub).saveStatus()).toBe(UiSaveState.Saved);
    });

    it('routes each panel save through its own handler', () => {
        const severityProject: Project = { ...project, idSeverityDefault: 5 };
        const issueTypeProject: Project = { ...project, idIssueTypeDefault: 5 };

        panel(ProjectSeverityStub).save.emit(severityProject);
        panel(ProjectIssueTypeStub).save.emit(issueTypeProject);

        expect(update).toHaveBeenCalledTimes(2);
        expect(update).toHaveBeenCalledWith(severityProject);
        expect(update).toHaveBeenCalledWith(issueTypeProject);
    });

    it('routes a migration through ProjectStore.setProject() and never PATCHes', () => {
        const migrated: Project = { ...project, idStateDefault: 4 };

        panel(ProjectStateStub).defaultMigrated.emit(migrated);

        expect(setProject).toHaveBeenCalledWith(migrated);
        expect(update).not.toHaveBeenCalled();
    });

    it('broadcasts the migrated project back to every panel via the store', () => {
        const migrated: Project = { ...project, idStateDefault: 4 };

        panel(ProjectSeverityStub).defaultMigrated.emit(migrated);
        project$.next(migrated);
        fixture.detectChanges();

        expect(panel(ProjectStateStub).project()).toBe(migrated);
        expect(panel(ProjectSeverityStub).project()).toBe(migrated);
        expect(panel(ProjectIssueTypeStub).project()).toBe(migrated);
    });

    it('pins the panel status to Error and shows the toast when a save fails', () => {
        update.mockReturnValue(throwError(() => new Error('boom')));

        panel(ProjectSeverityStub).save.emit({ ...project, idSeverityDefault: 1 });
        fixture.detectChanges();

        expect(showError).toHaveBeenCalledWith('PROJECT.SAVE_ERROR');
        expect(panel(ProjectSeverityStub).saveStatus()).toBe(UiSaveState.Error);
    });
});

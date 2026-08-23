import {
    ChangeDetectionStrategy,
    Component,
    computed,
    inject,
    input,
    OnDestroy,
    OnInit,
    signal
} from '@angular/core';
import { FormControl, FormGroup, NonNullableFormBuilder } from '@angular/forms';
import { Subscription } from 'rxjs';
import { filter, map } from 'rxjs/operators';
import cloneDeep from 'lodash-es/cloneDeep';
import { CdkDragDrop, CdkDragEnd, moveItemInArray } from '@angular/cdk/drag-drop';
import { Project } from 'src/app/project/model/project.model';
import { ProjectService } from 'src/app/project/project.service';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { WindowService } from 'src/app/shared/window/window.service';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';
import { IssueTypeApi } from '../../api/issue-type.api.service';
import { IssueType } from '../../model/issue-type.model';
import { IssueTypeUsage } from '../../model/issue-type-usage.model';
import { IssueTypeStore } from '../../store/issue-type.store';
import {
    DeleteMigrationOption,
    DeleteMigrationUsageItem
} from 'src/app/shared/components/delete-migration-dialog/delete-migration-option.model';
import { IssueTypeFormWindowComponent } from '../issue-type-form-window/issue-type-form-window.component';

interface ProjectIssueTypeForm {
    idIssueTypeDefault: FormControl<number | null>;
}

@Component({
    selector: 'app-project-issue-type',
    templateUrl: './project-issue-type.component.html',
    providers: [WindowService],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ProjectIssueTypeComponent implements OnInit, OnDestroy {
    private readonly i18n = inject(I18nService);
    private readonly fb = inject(NonNullableFormBuilder);
    private readonly sIssueType = inject(IssueTypeApi);
    private readonly sProject = inject(ProjectService);
    private readonly sWindow = inject(WindowService);
    private readonly issueTypeStore = inject(IssueTypeStore);

    public readonly project = input.required<Project>();

    protected readonly issueTypes = signal<IssueType[]>([]);
    protected readonly defaultSaveStatus = signal<UiSaveState>(UiSaveState.Idle);
    protected form!: FormGroup<ProjectIssueTypeForm>;

    private readonly subscription = new Subscription();

    public ngOnInit(): void {
        this.sIssueType
            .load$()
            .pipe(map(types => types.filter(t => t.idProject === this.project().idProject)))
            .subscribe(types => this.issueTypes.set(types));

        this.form = this.fb.group<ProjectIssueTypeForm>({
            idIssueTypeDefault: this.fb.control<number | null>(
                this.project().idIssueTypeDefault ?? null
            )
        });

        this.subscription.add(
            this.form.valueChanges
                .pipe(filter(() => this.form.valid))
                .subscribe(() => this.onProjectSave())
        );
    }

    public ngOnDestroy(): void {
        this.subscription.unsubscribe();
    }

    protected get idIssueTypeDefaultControl(): FormControl<number | null> {
        return this.form.controls.idIssueTypeDefault;
    }

    protected onProjectSave(): void {
        const project: Project = cloneDeep(this.project());
        project.idIssueTypeDefault = this.idIssueTypeDefaultControl.value;
        this.defaultSaveStatus.set(UiSaveState.Saving);
        this.sProject.updateProject(project).subscribe({
            next: savedProject => {
                this.project().idIssueTypeDefault = savedProject.idIssueTypeDefault;
                this.defaultSaveStatus.set(UiSaveState.Saved);
            },
            error: () => this.defaultSaveStatus.set(UiSaveState.Error)
        });
    }

    public onNewIssueType(): void {
        this.sWindow
            .open<IssueType | null>(IssueTypeFormWindowComponent, {
                header: this.i18n.instant('ISSUE_TYPE.NEW'),
                data: { project: this.project() }
            })
            .onClose.subscribe(savedIssueType => {
                if (!savedIssueType) {
                    return;
                }
                this.issueTypes.update(types => [...types, savedIssueType]);
                this.issueTypeStore.load();
            });
    }

    protected onEditIssueType(issueType: IssueType): void {
        this.sWindow
            .open<IssueType | null>(IssueTypeFormWindowComponent, {
                header: this.i18n.instant('ISSUE_TYPE.EDIT'),
                data: { project: this.project(), issueType }
            })
            .onClose.subscribe(savedIssueType => {
                if (!savedIssueType) {
                    return;
                }
                this.replaceIssueType(savedIssueType);
                this.issueTypeStore.load();
            });
    }

    protected readonly isDeleteDialogVisible = signal(false);
    protected readonly isDeleting = signal(false);
    protected readonly deleteTarget = signal<IssueType | null>(null);
    protected readonly deleteUsage = signal<IssueTypeUsage | null>(null);

    protected readonly deleteOptions = computed<DeleteMigrationOption[]>(() => {
        const target = this.deleteTarget();
        return this.issueTypes()
            .filter(t => t.idIssueType !== target?.idIssueType)
            .map(t => ({ id: t.idIssueType, label: t.name }));
    });

    protected readonly hasDeleteUsage = computed(() => {
        const usage = this.deleteUsage();
        return !!usage && (usage.issues > 0 || usage.isProjectDefault);
    });

    protected readonly deleteUsageItems = computed<DeleteMigrationUsageItem[]>(() => {
        const usage = this.deleteUsage();
        if (!usage) {
            return [];
        }
        const parts: DeleteMigrationUsageItem[] = [];
        if (usage.issues === 1) {
            parts.push({ key: 'ISSUE_TYPE.DELETE_USAGE.ONE' });
        } else if (usage.issues > 1) {
            parts.push({
                key: 'ISSUE_TYPE.DELETE_USAGE.MANY',
                params: { count: usage.issues }
            });
        }
        if (usage.isProjectDefault) {
            parts.push({ key: 'ISSUE_TYPE.DELETE_USAGE.DEFAULT' });
        }
        return parts;
    });

    protected onDeleteIssueType(issueType: IssueType): void {
        this.deleteTarget.set(issueType);
        this.deleteUsage.set(null);
        this.sIssueType.usage$(this.project().idProject, issueType.idIssueType).subscribe(usage => {
            this.deleteUsage.set(usage);
            this.isDeleteDialogVisible.set(true);
        });
    }

    protected onConfirmDelete(choice: { migrateTo: number | null }): void {
        const target = this.deleteTarget();
        if (!target) {
            return;
        }

        const intent = this.hasDeleteUsage() ? choice : undefined;
        this.isDeleting.set(true);
        this.sIssueType.delete$(this.project().idProject, target.idIssueType, intent).subscribe({
            next: () => {
                this.isDeleting.set(false);
                this.isDeleteDialogVisible.set(false);
                this.removeIssueType(target);
                this.issueTypeStore.load();
                if (this.project().idIssueTypeDefault === target.idIssueType) {
                    this.project().idIssueTypeDefault = choice.migrateTo;
                    this.form.patchValue(
                        { idIssueTypeDefault: choice.migrateTo },
                        { emitEvent: false }
                    );
                }
            },
            error: () => this.isDeleting.set(false)
        });
    }

    protected onReorder(evt: CdkDragDrop<IssueType[]>): void {
        const before = this.issueTypes();
        const reordered = [...before];
        moveItemInArray(reordered, evt.previousIndex, evt.currentIndex);
        const renumbered = reordered.map((type, index) => ({ ...type, orderRank: index + 1 }));
        this.issueTypes.set(renumbered);
        this.sIssueType.update$(renumbered[evt.currentIndex]).subscribe({
            next: () => this.issueTypeStore.load(),
            error: () => this.issueTypes.set(before)
        });
    }

    protected onRowPointerDown(event: PointerEvent): void {
        this.setRowWidths(event.currentTarget as HTMLElement, true);
    }

    protected onRowPointerUp(event: PointerEvent): void {
        this.setRowWidths(event.currentTarget as HTMLElement, false);
    }

    protected onDragEnded(event: CdkDragEnd): void {
        this.setRowWidths(event.source.element.nativeElement, false);
    }

    private setRowWidths(row: HTMLElement, snapshot: boolean): void {
        for (const cell of Array.from(row.children)) {
            const el = cell as HTMLElement;
            el.style.width = snapshot ? `${el.offsetWidth}px` : '';
        }
    }

    private replaceIssueType(issueType: IssueType): void {
        this.issueTypes.update(types => {
            const idx = types.findIndex(t => t.idIssueType === issueType.idIssueType);
            if (idx !== -1) {
                const updated = [...types];
                updated[idx] = issueType;
                return updated;
            }
            return types;
        });
    }

    private removeIssueType(issueType: IssueType): void {
        this.issueTypes.update(types => types.filter(t => t.idIssueType !== issueType.idIssueType));
    }
}

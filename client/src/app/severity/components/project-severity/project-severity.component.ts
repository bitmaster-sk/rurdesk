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
import { FormBuilder, FormControl, FormGroup } from '@angular/forms';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { Subscription } from 'rxjs';
import { Project } from 'src/app/project/model/project.model';
import { ProjectService } from 'src/app/project/project.service';
import { WindowService } from 'src/app/shared/window/window.service';
import { IssueSeverity } from '../../model/issue-severity.model';
import { SeverityFormWindowComponent } from '../severity-form-window/severity-form-window.component';
import { SeverityApi } from '../../api/severity.api.service';
import { SeverityUsage } from '../../model/severity-usage.model';
import cloneDeep from 'lodash-es/cloneDeep';
import { filter, map } from 'rxjs/operators';
import { SeverityStore } from '../../store/severity.store';
import { CdkDragDrop, CdkDragEnd, moveItemInArray } from '@angular/cdk/drag-drop';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';

@Component({
    selector: 'app-project-severity',
    templateUrl: './project-severity.component.html',
    providers: [WindowService],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ProjectSeverityComponent implements OnInit, OnDestroy {
    private readonly i18n = inject(I18nService);
    private readonly fb = inject(FormBuilder);
    private readonly sSeverity = inject(SeverityApi);
    private readonly sProject = inject(ProjectService);
    private readonly sWindow = inject(WindowService);
    private readonly severityStore = inject(SeverityStore);

    public readonly project = input.required<Project>();

    protected readonly severities = signal<IssueSeverity[]>([]);
    protected readonly defaultSaveStatus = signal<UiSaveState>(UiSaveState.Idle);
    protected form!: FormGroup<{
        idSeverityDefault: FormControl<number | null>;
    }>;

    private readonly subscription = new Subscription();

    public ngOnInit(): void {
        this.sSeverity
            .load$()
            .pipe(
                map(severities => severities.filter(s => s.idProject === this.project().idProject))
            )
            .subscribe(severities => this.severities.set(severities));

        this.form = this.fb.group({
            idSeverityDefault: this.fb.control<number | null>(
                this.project().idSeverityDefault ?? null
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

    protected get idSeverityDefaultControl(): FormControl<number | null> {
        return this.form.controls.idSeverityDefault;
    }

    protected onProjectSave(): void {
        const project: Project = cloneDeep(this.project());
        project.idSeverityDefault = this.form.value.idSeverityDefault ?? null;
        this.defaultSaveStatus.set(UiSaveState.Saving);
        this.sProject.updateProject(project).subscribe({
            next: savedProject => {
                this.project().idSeverityDefault = savedProject.idSeverityDefault;
                this.defaultSaveStatus.set(UiSaveState.Saved);
            },
            error: () => this.defaultSaveStatus.set(UiSaveState.Error)
        });
    }

    public onNewSeverity(): void {
        this.sWindow
            .open<IssueSeverity | null>(SeverityFormWindowComponent, {
                header: this.i18n.instant('SEVERITY.NEW'),
                data: { project: this.project() }
            })
            .onClose.subscribe(savedSeverity => {
                if (!savedSeverity) {
                    return;
                }
                this.severities.update(severities => [...severities, savedSeverity]);
                this.severityStore.load();
            });
    }

    protected onEditSeverity(severity: IssueSeverity): void {
        this.sWindow
            .open<IssueSeverity | null>(SeverityFormWindowComponent, {
                header: this.i18n.instant('SEVERITY.EDIT'),
                data: { project: this.project(), severity }
            })
            .onClose.subscribe(savedSeverity => {
                if (!savedSeverity) {
                    return;
                }
                this.replaceSeverity(savedSeverity);
                this.severityStore.load();
            });
    }

    protected readonly isDeleteDialogVisible = signal(false);
    protected readonly isDeleting = signal(false);
    protected readonly deleteTarget = signal<IssueSeverity | null>(null);
    protected readonly deleteUsage = signal<SeverityUsage | null>(null);

    protected readonly deleteOptions = computed<IssueSeverity[]>(() => {
        const target = this.deleteTarget();
        return this.severities().filter(s => s.idSeverity !== target?.idSeverity);
    });

    protected readonly hasDeleteUsage = computed(() => {
        const usage = this.deleteUsage();
        return !!usage && (usage.issues > 0 || usage.isProjectDefault);
    });

    protected readonly deleteUsageItems = computed(() => {
        const usage = this.deleteUsage();
        if (!usage) {
            return [];
        }
        const parts: string[] = [];
        if (usage.issues === 1) {
            parts.push(this.i18n.instant('SEVERITY.DELETE_USAGE.ONE'));
        } else if (usage.issues > 1) {
            parts.push(this.i18n.instant('SEVERITY.DELETE_USAGE.MANY', { count: usage.issues }));
        }
        if (usage.isProjectDefault) {
            parts.push(this.i18n.instant('SEVERITY.DELETE_USAGE.DEFAULT'));
        }
        return parts;
    });

    protected onDeleteSeverity(severity: IssueSeverity): void {
        this.deleteTarget.set(severity);
        this.deleteUsage.set(null);
        this.sSeverity.usage$(this.project().idProject, severity.idSeverity).subscribe(usage => {
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
        this.sSeverity.delete$(this.project().idProject, target.idSeverity, intent).subscribe({
            next: () => {
                this.isDeleting.set(false);
                this.isDeleteDialogVisible.set(false);
                this.removeSeverity(target);
                this.severityStore.load();
                // refresh the local default so a later save doesn't PATCH the stale id back
                if (this.project().idSeverityDefault === target.idSeverity) {
                    this.project().idSeverityDefault = choice.migrateTo;
                    this.form.patchValue(
                        { idSeverityDefault: choice.migrateTo },
                        { emitEvent: false }
                    );
                }
            },
            error: () => this.isDeleting.set(false) // dialog stays open, toast comes from ErrorInterceptor
        });
    }

    protected onReorder(evt: CdkDragDrop<IssueSeverity[]>): void {
        // CDK does NOT mutate the array — reorder a copy and set the signal first,
        // THEN read the moved item at its new index.
        const reordered = [...this.severities()];
        moveItemInArray(reordered, evt.previousIndex, evt.currentIndex);
        this.severities.set(reordered);
        const moved = reordered[evt.currentIndex];
        moved.orderRank = evt.currentIndex + 1;
        this.sSeverity.update$(moved).subscribe();
    }

    // Snapshot cell widths on pointerdown so the detached drag-preview <tr> keeps
    // table layout (CDK clones the row at drag-start, out of table context). Cleared
    // on both cdkDragEnded (drag case) AND pointerup (plain click never drags, so
    // cdkDragEnded won't fire — without this the widths would freeze the columns).
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

    private replaceSeverity(severity: IssueSeverity): void {
        this.severities.update(severities => {
            const idx = severities.findIndex(s => s.idSeverity === severity.idSeverity);
            if (idx !== -1) {
                const updated = [...severities];
                updated[idx] = severity;
                return updated;
            }
            return severities;
        });
    }

    private removeSeverity(severity: IssueSeverity): void {
        this.severities.update(severities =>
            severities.filter(s => s.idSeverity !== severity.idSeverity)
        );
    }
}

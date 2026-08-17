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
import { FormBuilder, FormGroup } from '@angular/forms';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { Project } from 'src/app/project/model/project.model';
import { ProjectService } from 'src/app/project/project.service';
import { WindowService } from 'src/app/shared/window/window.service';
import { StateApi } from '../../api/state.api.service';
import { StateUsage } from '../../model/state-usage.model';
import cloneDeep from 'lodash-es/cloneDeep';
import { Subscription } from 'rxjs';
import { filter, map } from 'rxjs/operators';
import { StateStore } from '../../store/state.store';
import { CdkDragDrop, CdkDragEnd, moveItemInArray } from '@angular/cdk/drag-drop';
import { StateFormWindowComponent } from '../state-form-window/state-form-window.component';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';

@Component({
    selector: 'app-project-state',
    templateUrl: './project-state.component.html',
    providers: [WindowService],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ProjectStateComponent implements OnInit, OnDestroy {
    private readonly i18n = inject(I18nService);
    private readonly fb = inject(FormBuilder);
    private readonly stateApi = inject(StateApi);
    private readonly sProject = inject(ProjectService);
    private readonly sWindow = inject(WindowService);
    private readonly stateStore = inject(StateStore);

    public readonly project = input.required<Project>();

    protected readonly states = signal<IssueState[]>([]);
    protected readonly defaultSaveStatus = signal<UiSaveState>(UiSaveState.Idle);
    protected form: FormGroup = new FormGroup({});

    private readonly subscription = new Subscription();

    public ngOnInit(): void {
        this.stateApi
            .load$()
            .pipe(map(states => states.filter(s => s.idProject === this.project().idProject)))
            .subscribe(states => this.states.set(states));

        this.form = this.fb.group({
            idStateDefault: this.fb.control(this.project().idStateDefault)
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

    protected onProjectSave(): void {
        const project: Project = cloneDeep(this.project());
        project.idStateDefault = this.form.value.idStateDefault;
        this.defaultSaveStatus.set(UiSaveState.Saving);
        this.sProject.updateProject(project).subscribe({
            next: savedProject => {
                this.project().idStateDefault = savedProject.idStateDefault;
                this.defaultSaveStatus.set(UiSaveState.Saved);
            },
            error: () => this.defaultSaveStatus.set(UiSaveState.Error)
        });
    }

    public onNewState(): void {
        this.sWindow
            .open(StateFormWindowComponent, {
                header: this.i18n.instant('STATE.NEW'),
                data: { project: this.project() }
            })
            .onClose.subscribe((savedState: IssueState) => {
                if (!savedState) {
                    return;
                }
                this.states.update(states => [...states, savedState]);
                this.stateStore.load();
            });
    }

    protected onEditState(state: IssueState): void {
        this.sWindow
            .open(StateFormWindowComponent, {
                header: this.i18n.instant('STATE.EDIT'),
                data: { project: this.project(), state }
            })
            .onClose.subscribe((savedState: IssueState) => {
                if (!savedState) {
                    return;
                }
                this.replaceState(savedState);
                this.stateStore.load();
            });
    }

    protected readonly isDeleteDialogVisible = signal(false);
    protected readonly isDeleting = signal(false);
    protected readonly deleteTarget = signal<IssueState | null>(null);
    protected readonly deleteUsage = signal<StateUsage | null>(null);

    protected readonly deleteOptions = computed<IssueState[]>(() => {
        const target = this.deleteTarget();
        return this.states().filter(s => s.idState !== target?.idState);
    });

    protected readonly hasDeleteUsage = computed(() => {
        const usage = this.deleteUsage();
        return !!usage && (usage.issues > 0 || usage.isProjectDefault || usage.agentPhases > 0);
    });

    protected readonly deleteUsageItems = computed(() => {
        const usage = this.deleteUsage();
        if (!usage) {
            return [];
        }
        const parts: string[] = [];
        if (usage.issues === 1) {
            parts.push(this.i18n.instant('STATE.DELETE_USAGE.ONE'));
        } else if (usage.issues > 1) {
            parts.push(this.i18n.instant('STATE.DELETE_USAGE.MANY', { count: usage.issues }));
        }
        if (usage.isProjectDefault) {
            parts.push(this.i18n.instant('STATE.DELETE_USAGE.DEFAULT'));
        }
        if (usage.agentPhases === 1) {
            parts.push(this.i18n.instant('STATE.DELETE_USAGE.PHASES_ONE'));
        } else if (usage.agentPhases > 1) {
            parts.push(
                this.i18n.instant('STATE.DELETE_USAGE.PHASES_MANY', { count: usage.agentPhases })
            );
        }
        return parts;
    });

    protected onDeleteState(state: IssueState): void {
        this.deleteTarget.set(state);
        this.deleteUsage.set(null);
        this.stateApi.usage$(this.project().idProject, state.idState).subscribe(usage => {
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
        this.stateApi.delete$(this.project().idProject, target.idState, intent).subscribe({
            next: () => {
                this.isDeleting.set(false);
                this.isDeleteDialogVisible.set(false);
                this.removeState(target);
                this.stateStore.load();
                // refresh the local default so a later save doesn't PATCH the stale id back
                if (this.project().idStateDefault === target.idState) {
                    this.project().idStateDefault = choice.migrateTo;
                    this.form.patchValue(
                        { idStateDefault: choice.migrateTo },
                        { emitEvent: false }
                    );
                }
            },
            error: () => this.isDeleting.set(false) // dialog stays open, toast comes from ErrorInterceptor
        });
    }

    protected onReorder(evt: CdkDragDrop<IssueState[]>): void {
        // CDK does NOT mutate the array — reorder a copy and set the signal first,
        // THEN read the moved item at its new index.
        const reordered = [...this.states()];
        moveItemInArray(reordered, evt.previousIndex, evt.currentIndex);
        this.states.set(reordered);
        const moved = reordered[evt.currentIndex];
        moved.orderRank = evt.currentIndex + 1;
        this.stateApi.update$(moved).subscribe();
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

    protected checkboxIcon(accepted?: boolean): string {
        if (accepted === true) {
            return 'circle-check';
        } else if (accepted === false) {
            return 'circle-x';
        }
        return '';
    }

    private replaceState(state: IssueState): void {
        this.states.update(states => {
            const idx = states.findIndex(s => s.idState === state.idState);
            if (idx !== -1) {
                const updated = [...states];
                updated[idx] = state;
                return updated;
            }
            return states;
        });
    }

    private removeState(state: IssueState): void {
        this.states.update(states => states.filter(s => s.idState !== state.idState));
    }
}

import {
    ChangeDetectionStrategy,
    Component,
    inject,
    input,
    OnDestroy,
    OnInit,
    signal
} from '@angular/core';
import { FormBuilder, FormControl, FormGroup } from '@angular/forms';
import { TranslateService } from '@ngx-translate/core';
import { Subscription } from 'rxjs';
import { Project } from 'src/app/project/model/project.model';
import { ProjectService } from 'src/app/project/project.service';
import { WindowService } from 'src/app/shared/window/window.service';
import { IssueSeverity } from '../../model/issue-severity.model';
import { SeverityFormWindowComponent } from '../severity-form-window/severity-form-window.component';
import { SeverityApi } from '../../api/severity.api.service';
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
    private readonly i18n = inject(TranslateService);
    private readonly fb = inject(FormBuilder);
    private readonly sSeverity = inject(SeverityApi);
    private readonly sProject = inject(ProjectService);
    private readonly sWindow = inject(WindowService);
    private readonly severityStore = inject(SeverityStore);

    public readonly project = input.required<Project>();

    protected readonly severities = signal<IssueSeverity[]>([]);
    protected readonly defaultSaveStatus = signal<UiSaveState>(UiSaveState.Idle);
    protected form: FormGroup = new FormGroup({});

    private readonly subscription = new Subscription();

    public ngOnInit(): void {
        this.sSeverity
            .load$()
            .pipe(
                map(severities => severities.filter(s => s.idProject === this.project().idProject))
            )
            .subscribe(severities => this.severities.set(severities));

        this.form = this.fb.group({
            idSeverityDefault: this.fb.control(this.project().idSeverityDefault)
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

    protected get idSeverityDefaultControl(): FormControl {
        return this.form.get('idSeverityDefault') as FormControl;
    }

    protected onProjectSave(): void {
        const project: Project = cloneDeep(this.project());
        project.idSeverityDefault = this.form.value.idSeverityDefault;
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
            .open(SeverityFormWindowComponent, {
                header: this.i18n.instant('SEVERITY.NEW'),
                data: { project: this.project() }
            })
            .onClose.subscribe((savedSeverity: IssueSeverity) => {
                if (!savedSeverity) {
                    return;
                }
                this.severities.update(severities => [...severities, savedSeverity]);
                this.severityStore.load();
            });
    }

    protected onEditSeverity(severity: IssueSeverity): void {
        this.sWindow
            .open(SeverityFormWindowComponent, {
                header: this.i18n.instant('SEVERITY.EDIT'),
                data: { project: this.project(), severity }
            })
            .onClose.subscribe((savedSeverity: IssueSeverity) => {
                if (!savedSeverity) {
                    return;
                }
                this.replaceSeverity(savedSeverity);
                this.severityStore.load();
            });
    }

    protected onConfirmDeleteSeverity(severity: IssueSeverity): void {
        this.sSeverity.delete$(this.project().idProject, severity.idSeverity).subscribe(() => {
            this.removeSeverity(severity);
            this.severityStore.load();
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

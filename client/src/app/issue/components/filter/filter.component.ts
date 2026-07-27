import { ChangeDetectionStrategy, Component, DestroyRef, OnInit, inject } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormBuilder, FormControl, FormGroup } from '@angular/forms';
import { Observable } from 'rxjs';
import { first, map, switchMap } from 'rxjs/operators';
import { User } from 'src/app/auth/model/user.model';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { ProjectStore } from 'src/app/project/project.store';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { StateStore } from 'src/app/state/store/state.store';
import { IssueFilterStore } from './issue-filter.store';

@Component({
    selector: 'app-filter',
    templateUrl: './filter.component.html',
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class FilterComponent implements OnInit {
    private readonly fb = inject(FormBuilder);
    private readonly severityStore = inject(SeverityStore);
    private readonly stateStore = inject(StateStore);
    private readonly projectMemberStore = inject(ProjectMemberStore);
    private readonly issueFilterStore = inject(IssueFilterStore);
    private readonly projectStore = inject(ProjectStore);
    private readonly destroyRef = inject(DestroyRef);

    public readonly severities$: Observable<IssueSeverity[]> = this.projectStore.project$.pipe(
        switchMap(project => this.severityStore.severitiesByProject$(project.idProject))
    );

    public readonly states$: Observable<IssueState[]> = this.projectStore.project$.pipe(
        switchMap(project => this.stateStore.statesByProject$(project.idProject))
    );

    public readonly users$: Observable<User[]> = this.projectMemberStore.users$;

    public readonly initialFilter$ = this.issueFilterStore.initialFilter$;

    public readonly isFilterInitialized$ = this.initialFilter$.pipe(map(filter => !!filter));

    public form: FormGroup = this.fb.group({
        idProject: this.fb.control(null),
        idsState: this.fb.control(null),
        stateUnset: this.fb.control(null),
        idsSeverity: this.fb.control(null),
        severityUnset: this.fb.control(null),
        idsAssignedTo: this.fb.control(null),
        assignedToUnset: this.fb.control(null),
        createAtRange: this.fb.control(null),
        updateAtRange: this.fb.control(null),
        title: this.fb.control(null)
    });

    public get stateUnsetControl(): FormControl {
        return this.form.get('stateUnset') as FormControl;
    }

    public get severityUnsetControl(): FormControl {
        return this.form.get('severityUnset') as FormControl;
    }

    public get assignedToUnsetControl(): FormControl {
        return this.form.get('assignedToUnset') as FormControl;
    }

    public ngOnInit(): void {
        this.form.valueChanges.pipe(takeUntilDestroyed(this.destroyRef)).subscribe(() => {
            const values = this.form.value;
            const [createAtFrom = null, createAtTo = null] = values.createAtRange ?? [];
            const [updateAtFrom = null, updateAtTo = null] = values.updateAtRange ?? [];
            this.issueFilterStore.setFilter({
                idProject: values.idProject,
                title: values.title,
                idsSeverity: values.idsSeverity,
                severityUnset: values.severityUnset,
                idsState: values.idsState,
                stateUnset: values.stateUnset,
                idsAssignedTo: values.idsAssignedTo,
                assignedToUnset: values.assignedToUnset,
                createAtFrom,
                createAtTo,
                updateAtFrom,
                updateAtTo
            });
        });

        this.initialFilter$.pipe(first()).subscribe(filter => {
            this.form.patchValue(
                {
                    idProject: filter.idProject,
                    title: filter.title,
                    idsSeverity: filter.idsSeverity,
                    severityUnset: filter.severityUnset,
                    idsState: filter.idsState,
                    stateUnset: filter.stateUnset,
                    idsAssignedTo: filter.idsAssignedTo,
                    assignedToUnset: filter.assignedToUnset,
                    createAtRange:
                        filter.createAtFrom || filter.createAtTo
                            ? [filter.createAtFrom, filter.createAtTo]
                            : null,
                    updateAtRange:
                        filter.updateAtFrom || filter.updateAtTo
                            ? [filter.updateAtFrom, filter.updateAtTo]
                            : null
                },
                { emitEvent: false }
            );
        });
    }
}

import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    OnInit,
    inject,
    signal
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormBuilder, FormControl, FormGroup } from '@angular/forms';
import { Observable, merge, of } from 'rxjs';
import { distinctUntilChanged, switchMap } from 'rxjs/operators';
import { User } from 'src/app/auth/model/user.model';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { ProjectStore } from 'src/app/project/project.store';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { StateStore } from 'src/app/state/store/state.store';
import { TranslateService } from '@ngx-translate/core';
import {
    UiDateRangePreset,
    UiDateRangeValue
} from 'src/app/ui/components/date-range-select/date-range-select.component';
import { IssuesFilterParams } from './issue-filter.entity';
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
    private readonly i18n = inject(TranslateService);

    public readonly severities$: Observable<IssueSeverity[]> = this.projectStore.project$.pipe(
        switchMap(project => this.severityStore.severitiesByProject$(project.idProject))
    );

    public readonly states$: Observable<IssueState[]> = this.projectStore.project$.pipe(
        switchMap(project => this.stateStore.statesByProject$(project.idProject))
    );

    public readonly users$: Observable<User[]> = this.projectMemberStore.users$;

    public form: FormGroup = this.fb.group({
        idsState: this.fb.control(null),
        stateUnset: this.fb.control(null),
        idsSeverity: this.fb.control(null),
        severityUnset: this.fb.control(null),
        idsAssignedTo: this.fb.control(null),
        assignedToUnset: this.fb.control(null),
        createAt: this.fb.control(null),
        updateAt: this.fb.control(null),
        title: this.fb.control(null)
    });

    /** Values ARE the API's duration grammar; 'Custom range' comes from the control. */
    private readonly datePresets: UiDateRangePreset[] = [
        { label: this.i18n.instant('FILTER.DATE.LAST_7_DAYS'), value: '7d' },
        { label: this.i18n.instant('FILTER.DATE.LAST_30_DAYS'), value: '30d' },
        { label: this.i18n.instant('FILTER.DATE.LAST_90_DAYS'), value: '90d' }
    ];

    // Reassigned from an async subscription under OnPush, so signals.
    protected readonly createAtPresets = signal<UiDateRangePreset[]>([...this.datePresets]);
    protected readonly updateAtPresets = signal<UiDateRangePreset[]>([...this.datePresets]);

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
        this.onFormChange();
        this.onFilterChange();
    }

    private onFormChange(): void {
        this.form.valueChanges.pipe(takeUntilDestroyed(this.destroyRef)).subscribe(() => {
            const values = this.form.value;
            const createAt = this.dateFilterFor(values.createAt);
            const updateAt = this.dateFilterFor(values.updateAt);
            // No idProject on purpose: the panel never edits it, and pushing its own copy
            // overwrote the store's with null whenever the panel mounted late.
            this.issueFilterStore.setFilter({
                title: values.title,
                idsSeverity: values.idsSeverity,
                severityUnset: values.severityUnset,
                idsState: values.idsState,
                stateUnset: values.stateUnset,
                idsAssignedTo: values.idsAssignedTo,
                assignedToUnset: values.assignedToUnset,
                createAtFrom: createAt.from,
                createAtTo: createAt.to,
                createAtWithin: createAt.within,
                updateAtFrom: updateAt.from,
                updateAtTo: updateAt.to,
                updateAtWithin: updateAt.within
            });
        });
    }

    // The board and gantt create this panel on demand, by which time initialFilter$ usually
    // replays nothing (the store's latest emission is a plain setFilter/setSprint), so
    // without the current filter up front the panel would open blank while one is active.
    private onFilterChange(): void {
        merge(of(this.issueFilterStore.getFilter()), this.issueFilterStore.initialFilter$)
            // When the latest emission was initial, both sources carry the same object.
            .pipe(distinctUntilChanged(), takeUntilDestroyed(this.destroyRef))
            .subscribe(issuesFilter => {
                if (issuesFilter) {
                    this.hydrate(issuesFilter);
                }
            });
    }

    /** `emitEvent: false` is what stops this looping back through valueChanges. */
    private hydrate(filter: IssuesFilterParams): void {
        this.createAtPresets.set(this.withPreset(this.datePresets, filter.createAtWithin));
        this.updateAtPresets.set(this.withPreset(this.datePresets, filter.updateAtWithin));
        this.form.patchValue(
            {
                title: filter.title,
                idsSeverity: filter.idsSeverity,
                severityUnset: filter.severityUnset,
                idsState: filter.idsState,
                stateUnset: filter.stateUnset,
                idsAssignedTo: filter.idsAssignedTo,
                assignedToUnset: filter.assignedToUnset,
                createAt: this.dateValueFor(filter, 'createAt'),
                updateAt: this.dateValueFor(filter, 'updateAt')
            },
            { emitEvent: false }
        );
    }

    /** Control value → filter params. */
    private dateFilterFor(value: UiDateRangeValue | null): {
        from: Date | null;
        to: Date | null;
        within: string | null;
    } {
        if (!value) {
            return { from: null, to: null, within: null };
        }
        if (value.preset) {
            return { from: null, to: null, within: value.preset };
        }
        return { from: value.from ?? null, to: value.to ?? null, within: null };
    }

    /** Filter params → control value. A window wins over absolute bounds. */
    private dateValueFor(
        filter: IssuesFilterParams,
        field: 'createAt' | 'updateAt'
    ): UiDateRangeValue | null {
        const within = field === 'createAt' ? filter.createAtWithin : filter.updateAtWithin;
        if (within) {
            return { preset: within };
        }
        const from = field === 'createAt' ? filter.createAtFrom : filter.updateAtFrom;
        const to = field === 'createAt' ? filter.createAtTo : filter.updateAtTo;
        return from || to ? { from: from ?? undefined, to: to ?? undefined } : null;
    }

    /** Adds a row for a value no preset lists (e.g. '1d8h6m' from a saved view or agent). */
    private withPreset(presets: UiDateRangePreset[], value?: string | null): UiDateRangePreset[] {
        if (!value || presets.some(preset => preset.value === value)) {
            return [...presets];
        }
        return [...presets, { label: value, value }];
    }
}

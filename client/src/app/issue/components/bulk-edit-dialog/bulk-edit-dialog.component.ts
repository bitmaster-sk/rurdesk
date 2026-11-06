import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    computed,
    inject,
    input,
    output,
    signal
} from '@angular/core';
import { takeUntilDestroyed, toObservable, toSignal } from '@angular/core/rxjs-interop';
import { combineLatest, first, switchMap } from 'rxjs';
import { User } from 'src/app/auth/model/user.model';
import { IssueType } from 'src/app/issue-type/model/issue-type.model';
import { IssueTypeStore } from 'src/app/issue-type/store/issue-type.store';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { StateStore } from 'src/app/state/store/state.store';
import { Tag } from 'src/app/tag/model/tag.model';
import { TagStore } from 'src/app/tag/store/tag.store';
import { UiToastService } from 'src/app/ui/service/ui-toast.service';
import { IssueBulkApi } from '../../api/issue-bulk.api.service';
import { TagPresence } from '../../constants/tag-presence.enum';
import { Issue } from '../../model/issue.model';
import { BulkEditIssueChangesReq } from '../../model/bulk-edit-issues.model';
import { BulkEditPlan } from '../../util/bulk-edit-plan';
import { ChangeRow } from './entity/bulk-edit-change-row.entity';

@Component({
    selector: 'app-bulk-edit-dialog',
    templateUrl: './bulk-edit-dialog.component.html',
    styleUrls: ['./bulk-edit-dialog.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class BulkEditDialogComponent {
    private static readonly UNASSIGNED_OPTION_ID = 0;

    private readonly stateStore = inject(StateStore);
    private readonly severityStore = inject(SeverityStore);
    private readonly issueTypeStore = inject(IssueTypeStore);
    private readonly memberStore = inject(ProjectMemberStore);
    private readonly tagStore = inject(TagStore);
    private readonly bulkApi = inject(IssueBulkApi);
    private readonly toast = inject(UiToastService);
    private readonly i18n = inject(I18nService);
    private readonly destroyRef = inject(DestroyRef);

    public readonly idProject = input.required<number>();
    public readonly issues = input.required<readonly Issue[]>();

    public readonly closed = output<void>();
    public readonly applied = output<Issue[]>();

    protected readonly Presence = TagPresence;

    protected readonly step = signal<'edit' | 'review'>('edit');
    protected readonly isSaving = signal(false);

    protected readonly idState = signal<number | null>(null);
    protected readonly idSeverity = signal<number | null>(null);
    protected readonly idIssueType = signal<number | null>(null);
    protected readonly assignee = signal<number | null | undefined>(undefined);
    protected readonly tagPresence = signal<ReadonlyMap<number, TagPresence>>(new Map());
    protected readonly initialTagPresence = signal<ReadonlyMap<number, TagPresence>>(new Map());

    private readonly lookups = toSignal(
        toObservable(this.idProject).pipe(
            switchMap(idProject =>
                combineLatest([
                    this.stateStore.states$,
                    this.severityStore.severities$,
                    this.issueTypeStore.issueTypes$,
                    this.memberStore.users$,
                    this.tagStore.tagsByProject$(idProject)
                ])
            )
        )
    );

    protected readonly states = computed<IssueState[]>(
        () => this.lookups()?.[0].filter(s => s.idProject === this.idProject()) ?? []
    );
    protected readonly severities = computed<IssueSeverity[]>(
        () => this.lookups()?.[1].filter(s => s.idProject === this.idProject()) ?? []
    );
    protected readonly issueTypes = computed<IssueType[]>(
        () => this.lookups()?.[2].filter(t => t.idProject === this.idProject()) ?? []
    );
    protected readonly users = computed<User[]>(() => this.lookups()?.[3] ?? []);
    protected readonly tags = computed<Tag[]>(() => this.lookups()?.[4] ?? []);

    protected readonly count = computed(() => this.issues().length);

    protected readonly assigneeOptions = computed<User[]>(() => [
        {
            idUser: BulkEditDialogComponent.UNASSIGNED_OPTION_ID,
            name: this.i18n.instant('ISSUE.BULK_EDIT.UNASSIGN'),
            email: '',
            colorAvatarBg: '#d4d4d8',
            avatarInitials: '–'
        },
        ...this.users().filter(user => user.isActive !== false && !user.isAgent)
    ]);

    protected readonly assigneeValue = computed(() => {
        const assignee = this.assignee();
        if (assignee === undefined) return null;
        return assignee ?? BulkEditDialogComponent.UNASSIGNED_OPTION_ID;
    });

    private readonly changes = computed<BulkEditIssueChangesReq>(() => {
        const changes: BulkEditIssueChangesReq = BulkEditPlan.tagChanges(
            this.initialTagPresence(),
            this.tagPresence()
        );
        const idState = this.idState();
        const idSeverity = this.idSeverity();
        const idIssueType = this.idIssueType();
        const assignee = this.assignee();
        if (idState !== null) changes.idState = idState;
        if (idSeverity !== null) changes.idSeverity = idSeverity;
        if (idIssueType !== null) changes.idIssueType = idIssueType;
        if (assignee !== undefined) changes.idUserAssigned = assignee;
        return changes;
    });

    protected readonly changeCount = computed(() => Object.keys(this.changes()).length);

    protected readonly changeRows = computed<ChangeRow[]>(() => {
        const changes = this.changes();
        const rows: ChangeRow[] = [];
        const state = this.states().find(s => s.idState === changes.idState);
        if (state) rows.push({ labelKey: 'ISSUE.QUICK_ACTIONS.STATE', value: state.name });
        const severity = this.severities().find(s => s.idSeverity === changes.idSeverity);
        if (severity) {
            rows.push({ labelKey: 'ISSUE.QUICK_ACTIONS.SEVERITY', value: severity.title });
        }
        const type = this.issueTypes().find(t => t.idIssueType === changes.idIssueType);
        if (type) rows.push({ labelKey: 'ISSUE.QUICK_ACTIONS.ISSUE_TYPE', value: type.name });
        if (changes.idUserAssigned !== undefined) {
            rows.push({
                labelKey: 'ISSUE.QUICK_ACTIONS.ASSIGNEE',
                value:
                    changes.idUserAssigned === null
                        ? this.i18n.instant('ISSUE.BULK_EDIT.UNASSIGN')
                        : (this.users().find(user => user.idUser === changes.idUserAssigned)
                              ?.name ?? '')
            });
        }
        if (changes.addIdsTag) {
            rows.push({
                labelKey: 'ISSUE.BULK_EDIT.ADD_TAGS',
                value: this.tagNames(changes.addIdsTag)
            });
        }
        if (changes.removeIdsTag) {
            rows.push({
                labelKey: 'ISSUE.BULK_EDIT.REMOVE_TAGS',
                value: this.tagNames(changes.removeIdsTag)
            });
        }
        return rows;
    });

    public constructor() {
        toObservable(this.lookups)
            .pipe(first(Boolean), takeUntilDestroyed(this.destroyRef))
            .subscribe(() => {
                const initial = BulkEditPlan.initialTagPresence(
                    this.issues(),
                    this.tags().map(tag => tag.idTag)
                );
                this.initialTagPresence.set(initial);
                this.tagPresence.set(initial);
            });
    }

    protected plural(key: string, count: number): string {
        return count === 1 ? `${key}_ONE` : key;
    }

    protected presenceOf(idTag: number): TagPresence {
        return this.tagPresence().get(idTag) ?? TagPresence.None;
    }

    protected tagCount(idTag: number): number {
        return BulkEditPlan.countWithTag(this.issues(), idTag);
    }

    protected tagChange(idTag: number): 'add' | 'remove' | null {
        const presence = this.presenceOf(idTag);
        if (presence === this.initialTagPresence().get(idTag)) return null;
        if (presence === TagPresence.All) return 'add';
        return presence === TagPresence.None ? 'remove' : null;
    }

    protected onTagToggle(idTag: number): void {
        const initial = this.initialTagPresence().get(idTag) ?? TagPresence.None;
        this.tagPresence.update(presenceByTag =>
            new Map(presenceByTag).set(
                idTag,
                BulkEditPlan.nextTagPresence(this.presenceOf(idTag), initial)
            )
        );
    }

    protected onTagKeydown(event: KeyboardEvent, idTag: number): void {
        if (event.key !== ' ' && event.key !== 'Enter') return;
        event.preventDefault();
        this.onTagToggle(idTag);
    }

    protected onAssigneePicked(idUser: number | null): void {
        if (idUser === null) {
            this.assignee.set(undefined);
            return;
        }
        this.assignee.set(idUser === BulkEditDialogComponent.UNASSIGNED_OPTION_ID ? null : idUser);
    }

    protected onReview(): void {
        if (this.changeCount() > 0) this.step.set('review');
    }

    protected onBack(): void {
        this.step.set('edit');
    }

    protected onCancel(): void {
        if (!this.isSaving()) this.closed.emit();
    }

    protected onApply(): void {
        if (this.isSaving()) return;
        const issues = this.issues();
        const entries = BulkEditPlan.toEntries(issues, this.changes());
        if (entries.length === 0) {
            this.closed.emit();
            return;
        }
        this.isSaving.set(true);
        this.bulkApi
            .update$(this.idProject(), entries)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: updated => {
                    this.isSaving.set(false);
                    this.toast.show({
                        severity: 'success',
                        detail: this.i18n.instant(
                            this.plural('ISSUE.BULK_EDIT.SUCCESS', issues.length),
                            {
                                count: issues.length
                            }
                        ),
                        life: 3000
                    });
                    this.applied.emit(updated);
                },
                error: () => this.isSaving.set(false)
            });
    }

    private tagNames(ids: number[]): string {
        const byId = new Map(this.tags().map(tag => [tag.idTag, tag.name]));
        return ids.map(id => byId.get(id) ?? `#${id}`).join(', ');
    }
}

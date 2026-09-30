import {
    ChangeDetectionStrategy,
    Component,
    OnInit,
    computed,
    inject,
    input,
    signal
} from '@angular/core';
import { CdkDragDrop, CdkDragEnd, moveItemInArray } from '@angular/cdk/drag-drop';
import { Project } from 'src/app/project/model/project.model';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { WindowService } from 'src/app/shared/window/window.service';
import {
    DeleteMigrationOption,
    DeleteMigrationUsageItem
} from 'src/app/shared/components/delete-migration-dialog/delete-migration-option.model';
import { CustomFieldApi } from '../../api/custom-field.api.service';
import { CustomField, CustomFieldOption } from '../../model/custom-field.model';
import { CustomFieldUsage } from '../../model/custom-field-usage.model';
import { CustomFieldStore } from '../../store/custom-field.store';
import {
    CustomFieldFormWindowComponent,
    CustomFieldWindowResult
} from '../custom-field-form-window/custom-field-form-window.component';

@Component({
    selector: 'app-project-custom-field',
    templateUrl: './project-custom-field.component.html',
    styleUrls: ['./project-custom-field.component.scss'],
    providers: [WindowService],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ProjectCustomFieldComponent implements OnInit {
    private readonly i18n = inject(I18nService);
    private readonly customFieldApi = inject(CustomFieldApi);
    private readonly sWindow = inject(WindowService);
    private readonly customFieldStore = inject(CustomFieldStore);

    public readonly project = input.required<Project>();

    protected readonly customFields = signal<CustomField[]>([]);

    protected readonly isDeleteDialogVisible = signal(false);
    protected readonly isDeleting = signal(false);
    protected readonly deleteTarget = signal<CustomField | null>(null);
    protected readonly deleteUsage = signal<CustomFieldUsage | null>(null);

    protected readonly isOptionDialogVisible = signal(false);
    protected readonly isResolvingOption = signal(false);
    protected readonly pendingField = signal<CustomField | null>(null);

    protected readonly hasDeleteUsage = computed(() => (this.deleteUsage()?.issues ?? 0) > 0);

    protected readonly deleteUsageItems = computed<DeleteMigrationUsageItem[]>(() => {
        const issues = this.deleteUsage()?.issues ?? 0;
        if (issues === 0) {
            return [];
        }
        return [{ key: 'CUSTOM_FIELD.DELETE_CONFIRM', params: { count: issues } }];
    });

    protected readonly optionUsageItems = computed<DeleteMigrationUsageItem[]>(() => [
        { key: 'CUSTOM_FIELD.OPTION_IN_USE' }
    ]);

    protected readonly optionTargets = computed<DeleteMigrationOption[]>(() =>
        (this.pendingField()?.options ?? []).map(option => ({
            id: option.idOption,
            label: option.label
        }))
    );

    public ngOnInit(): void {
        this.customFieldStore
            .customFieldsByProject$(this.project().idProject)
            .subscribe(fields => this.customFields.set(fields));
        this.customFieldStore.load();
    }

    public onNewCustomField(): void {
        this.sWindow
            .open<CustomFieldWindowResult | null>(CustomFieldFormWindowComponent, {
                header: this.i18n.instant('CUSTOM_FIELD.NEW'),
                data: { project: this.project() }
            })
            .onClose.subscribe(result => {
                if (result?.saved) {
                    this.customFieldStore.load();
                }
            });
    }

    protected onEditCustomField(customField: CustomField): void {
        this.sWindow
            .open<CustomFieldWindowResult | null>(CustomFieldFormWindowComponent, {
                header: this.i18n.instant('CUSTOM_FIELD.EDIT'),
                data: { project: this.project(), customField }
            })
            .onClose.subscribe(result => {
                if (result?.saved) {
                    this.customFieldStore.load();
                    return;
                }
                if (result?.optionConflict) {
                    this.resolveOptionConflict(result.optionConflict);
                }
            });
    }

    private resolveOptionConflict(field: CustomField): void {
        const added = field.options.filter(option => option.idOption === 0);
        const stored = this.customFields().find(
            candidate => candidate.idCustomField === field.idCustomField
        );
        if (added.length === 0 || !stored) {
            this.openOptionDialog(field);
            return;
        }
        this.saveAddedOptions(field, added, stored);
    }

    // A migration target needs a real id, so the added options are saved on their own
    // first. This request removes nothing, which is why it cannot hit the conflict again.
    private saveAddedOptions(
        field: CustomField,
        added: CustomFieldOption[],
        stored: CustomField
    ): void {
        const kept = field.options.filter(option => option.idOption !== 0);
        const keptIds = new Set(kept.map(option => option.idOption));
        const removed = stored.options.filter(option => !keptIds.has(option.idOption));
        const sentIds = new Set([...kept, ...removed].map(option => option.idOption));

        this.isResolvingOption.set(true);
        this.customFieldApi
            .update$(
                { ...field, options: [...kept, ...added, ...removed] },
                { withOptions: true, withDefaultValue: true }
            )
            .subscribe({
                next: saved => {
                    this.isResolvingOption.set(false);
                    this.customFieldStore.load();
                    this.openOptionDialog({
                        ...field,
                        options: this.withSavedIds(field.options, saved, sentIds)
                    });
                },
                error: () => this.isResolvingOption.set(false)
            });
    }

    private withSavedIds(
        intended: CustomFieldOption[],
        saved: CustomField,
        sentIds: Set<number>
    ): CustomFieldOption[] {
        const fresh = saved.options.filter(option => !sentIds.has(option.idOption));
        let next = 0;
        return intended.map(option =>
            option.idOption === 0 && next < fresh.length ? fresh[next++] : option
        );
    }

    private openOptionDialog(field: CustomField): void {
        this.pendingField.set(field);
        this.isOptionDialogVisible.set(true);
    }

    protected onConfirmOptionIntent(choice: { migrateTo: number | null }): void {
        const field = this.pendingField();
        if (!field) {
            return;
        }
        this.isResolvingOption.set(true);
        this.customFieldApi
            .update$(field, {
                withOptions: true,
                withDefaultValue: true,
                intent:
                    choice.migrateTo === null
                        ? { deleteOptionValues: true }
                        : { migrateOptionTo: choice.migrateTo }
            })
            .subscribe({
                next: () => {
                    this.isResolvingOption.set(false);
                    this.isOptionDialogVisible.set(false);
                    this.customFieldStore.load();
                },
                error: () => this.isResolvingOption.set(false)
            });
    }

    protected onArchive(customField: CustomField, isArchived: boolean): void {
        this.customFieldApi
            .archive$(customField, isArchived)
            .subscribe(() => this.customFieldStore.load());
    }

    protected onDeleteCustomField(customField: CustomField): void {
        this.deleteTarget.set(customField);
        this.deleteUsage.set(null);
        this.customFieldApi
            .usage$(this.project().idProject, customField.idCustomField)
            .subscribe(usage => {
                this.deleteUsage.set(usage);
                this.isDeleteDialogVisible.set(true);
            });
    }

    protected onConfirmDelete(): void {
        const target = this.deleteTarget();
        if (!target) {
            return;
        }
        this.isDeleting.set(true);
        this.customFieldApi
            .delete$(this.project().idProject, target.idCustomField, this.hasDeleteUsage())
            .subscribe({
                next: () => {
                    this.isDeleting.set(false);
                    this.isDeleteDialogVisible.set(false);
                    this.customFieldStore.load();
                },
                error: () => this.isDeleting.set(false)
            });
    }

    protected onReorder(evt: CdkDragDrop<CustomField[]>): void {
        const before = this.customFields();
        const reordered = [...before];
        moveItemInArray(reordered, evt.previousIndex, evt.currentIndex);
        const renumbered = reordered.map((field, index) => ({ ...field, orderRank: index + 1 }));
        this.customFields.set(renumbered);
        this.customFieldApi.update$(renumbered[evt.currentIndex]).subscribe({
            next: () => this.customFieldStore.load(),
            error: () => this.customFields.set(before)
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
}

import {
    ChangeDetectionStrategy,
    Component,
    OnInit,
    computed,
    effect,
    inject,
    input,
    linkedSignal,
    output,
    signal
} from '@angular/core';
import { CustomFieldType } from 'src/app/custom-field/constants/custom-field-type.enum';
import { CustomFieldValueConverter } from 'src/app/custom-field/converter/custom-field-value.converter';
import { CustomField, CustomFieldValue } from 'src/app/custom-field/model/custom-field.model';
import { CustomFieldStore } from 'src/app/custom-field/store/custom-field.store';
import { IssueApi } from 'src/app/issue/api/issue.api.service';
import { Issue } from 'src/app/issue/model/issue.model';
import { Project } from 'src/app/project/model/project.model';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { ApiError } from 'src/app/shared/model/api-error.model';
import { UiToastService } from 'src/app/ui/service/ui-toast.service';

@Component({
    selector: 'app-issue-custom-fields',
    templateUrl: './issue-custom-fields.component.html',
    styleUrls: ['./issue-custom-fields.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class IssueCustomFieldsComponent implements OnInit {
    private readonly issueApi = inject(IssueApi);
    private readonly customFieldStore = inject(CustomFieldStore);
    private readonly toast = inject(UiToastService);
    private readonly i18n = inject(I18nService);

    public readonly issue = input.required<Issue | null | undefined>();
    public readonly project = input.required<Project | null | undefined>();
    public readonly isNewIssue = input(false);

    public readonly valuesChange = output<Record<string, CustomFieldValue>>();
    public readonly missingRequiredChange = output<boolean>();

    private readonly definitions = signal<CustomField[]>([]);

    protected readonly fieldTypes = CustomFieldType;

    /** Only a new task gets defaults; a saved one keeps what it was saved with, blanks too. */
    private readonly defaults = computed<Record<string, CustomFieldValue>>(() => {
        if (!this.isNewIssue()) {
            return {};
        }
        const seeded: Record<string, CustomFieldValue> = {};
        for (const field of this.definitions()) {
            if (!field.archivedAt && field.defaultValue !== null) {
                seeded[field.key] = field.defaultValue;
            }
        }
        return seeded;
    });

    /** Re-seeds whenever the bound issue emits again: the detail page hands over a new
     *  object on every websocket notice. */
    protected readonly values = linkedSignal<
        {
            stored: Record<string, CustomFieldValue> | undefined;
            seeded: Record<string, CustomFieldValue>;
        },
        Record<string, CustomFieldValue>
    >({
        source: () => ({ stored: this.issue()?.customFields, seeded: this.defaults() }),
        computation: source => ({ ...source.seeded, ...(source.stored ?? {}) })
    });

    /** An archived field stays visible while the task holds a value, or the value
     *  would look lost. */
    protected readonly fields = computed(() =>
        this.definitions().filter(
            field => !field.archivedAt || this.values()[field.key] !== undefined
        )
    );

    /** Never convert per call in the template: a fresh Date on every check makes ngModel
     *  see a new value forever, which is an endless change detection loop. */
    private readonly dateValues = computed<Record<string, Date | null>>(() => {
        const dates: Record<string, Date | null> = {};
        for (const field of this.fields()) {
            if (field.fieldType === CustomFieldType.Date) {
                dates[field.key] = CustomFieldValueConverter.toDate(
                    this.values()[field.key] ?? null
                );
            }
        }
        return dates;
    });

    protected readonly missingRequiredKeys = computed(() =>
        this.fields()
            .filter(field => this.isRequired(field) && this.isEmpty(this.values()[field.key]))
            .map(field => field.key)
    );

    public constructor() {
        effect(() => this.missingRequiredChange.emit(this.missingRequiredKeys().length > 0));
        effect(() => {
            if (this.isNewIssue()) {
                this.valuesChange.emit(this.values());
            }
        });
    }

    public ngOnInit(): void {
        const idProject = this.project()?.idProject;
        if (!idProject) {
            return;
        }
        this.customFieldStore
            .customFieldsByProject$(idProject)
            .subscribe(fields => this.definitions.set(fields));
        this.customFieldStore.ensureLoaded();
    }

    protected isReadOnly(field: CustomField): boolean {
        return !!field.archivedAt;
    }

    protected isRequired(field: CustomField): boolean {
        return field.isRequired && !field.archivedAt;
    }

    /** True where the rule started after the task existed, so the task is outside it. */
    protected predatesRequirement(field: CustomField): boolean {
        if (this.isNewIssue() || !field.requiredSince) {
            return false;
        }
        const createdAt = this.issue()?.createAt;
        if (!createdAt) {
            return false;
        }
        return new Date(createdAt).getTime() < new Date(field.requiredSince).getTime();
    }

    protected isMissing(field: CustomField): boolean {
        return this.isRequired(field) && this.isEmpty(this.values()[field.key]);
    }

    protected valueOf(field: CustomField): CustomFieldValue {
        return this.values()[field.key] ?? null;
    }

    protected dateValueOf(field: CustomField): Date | null {
        return this.dateValues()[field.key] ?? null;
    }

    protected onInputBlur(field: CustomField, event: Event): void {
        this.onValueChange(field, (event.target as HTMLInputElement).value);
    }

    protected onValueChange(field: CustomField, raw: unknown): void {
        const previous = this.valueOf(field);
        const next = CustomFieldValueConverter.toValue(field.fieldType, raw);
        if (next === previous) {
            return;
        }
        this.values.update(values => ({ ...values, [field.key]: next }));
        if (!this.isNewIssue()) {
            this.save(field, previous, next);
        }
    }

    private isEmpty(value: CustomFieldValue | undefined): boolean {
        return value === null || value === undefined || value === '';
    }

    private save(field: CustomField, previous: CustomFieldValue, next: CustomFieldValue): void {
        const issue = this.issue();
        const idProject = this.project()?.idProject;
        if (!issue || !idProject) {
            return;
        }
        this.issueApi
            .update$(idProject, issue.idIssuePublic, { customFields: { [field.key]: next } })
            .subscribe({
                error: (error: unknown) => {
                    this.values.update(values => ({ ...values, [field.key]: previous }));
                    // ErrorInterceptor already toasted what the API said; a second,
                    // vaguer toast would bury the real reason.
                    if (!ApiError.translateKeyOf(error)) {
                        this.toast.show({
                            severity: 'error',
                            detail: this.i18n.instant('CUSTOM_FIELD.SAVE_FAILED')
                        });
                    }
                }
            });
    }
}

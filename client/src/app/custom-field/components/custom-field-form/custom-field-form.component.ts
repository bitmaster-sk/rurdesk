import {
    ChangeDetectionStrategy,
    Component,
    OnInit,
    computed,
    effect,
    inject,
    input,
    output,
    signal
} from '@angular/core';
import { FormControl, FormGroup, NonNullableFormBuilder, Validators } from '@angular/forms';
import { CustomFieldType } from '../../constants/custom-field-type.enum';
import { CustomFieldKeyConverter } from '../../converter/custom-field-key.converter';
import { CustomField, CustomFieldOption, CustomFieldValue } from '../../model/custom-field.model';

interface CustomFieldForm {
    name: FormControl<string>;
    key: FormControl<string>;
    fieldType: FormControl<CustomFieldType>;
    isRequired: FormControl<boolean>;
}

export interface CustomFieldFormValue {
    field: CustomField;
    isNew: boolean;
    /** Value to put on existing tasks that have none; undefined must leave them blank. */
    backfill?: CustomFieldValue;
}

@Component({
    selector: 'app-custom-field-form',
    templateUrl: './custom-field-form.component.html',
    styleUrls: ['./custom-field-form.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class CustomFieldFormComponent implements OnInit {
    public readonly customField = input.required<Partial<CustomField>>();

    /** Tasks without a value for this field; null when it is not known yet (new field). */
    public readonly issuesMissing = input<number | null>(null);

    /** Archived fields included: an archived field still holds values, so its key is taken. */
    public readonly projectFields = input<CustomField[]>([]);

    public readonly save = output<CustomFieldFormValue>();

    public readonly cancelled = output<void>();

    protected form!: FormGroup<CustomFieldForm>;

    protected readonly options = signal<CustomFieldOption[]>([]);

    protected readonly fieldTypes: CustomFieldType[] = [
        CustomFieldType.Text,
        CustomFieldType.Number,
        CustomFieldType.Date,
        CustomFieldType.Select,
        CustomFieldType.Boolean
    ];

    protected readonly isNew = computed(() => !this.customField().idCustomField);

    private readonly fb = inject(NonNullableFormBuilder);

    protected readonly selectedType = signal<CustomFieldType>(CustomFieldType.Text);

    protected readonly backfill = signal<CustomFieldValue>(null);

    protected readonly defaultValue = signal<CustomFieldValue>(null);

    protected readonly isRequiredChecked = signal(false);

    protected readonly nameValue = signal('');

    protected readonly keyValue = signal('');

    /** Typing a key by hand stops it following the name; the key is immutable once saved. */
    protected readonly isKeyEdited = signal(false);

    protected readonly isKeyRevealed = signal(false);

    protected readonly conflictingField = computed(
        () => this.projectFields().find(field => this.isSameKey(field, this.keyValue())) ?? null
    );

    /** A name with no Latin letters yields no key, so the user has to write one. */
    protected readonly isKeyUngenerated = computed(
        () => this.isNew() && this.nameValue().trim() !== '' && this.keyValue() === ''
    );

    protected readonly isKeyEditable = computed(
        () =>
            this.isNew() &&
            (this.isKeyRevealed() || this.isKeyUngenerated() || !!this.conflictingField())
    );

    protected readonly isKeyLocked = computed(() => this.isNew() && !this.isKeyEditable());

    /** A brand-new select field has no saved option ids to point a default at. */
    protected readonly isSelectType = computed(
        () => this.selectedType() === CustomFieldType.Select
    );

    protected readonly isDefaultOffered = computed(() => !(this.isNew() && this.isSelectType()));

    protected readonly isBackfillOffered = computed(() => {
        if (!this.isRequiredChecked()) {
            return false;
        }
        if (this.isNew() && this.isSelectType()) {
            return false;
        }
        const missing = this.issuesMissing();
        return missing === null || missing > 0;
    });

    /** A freshly typed option has no id yet, so it cannot be sent as a value. */
    protected readonly savedOptions = computed(() =>
        this.options().filter(option => option.idOption !== 0)
    );

    public constructor() {
        // The definitions can land after the form was built, and a validator never
        // re-runs on its own when the list it reads changes.
        effect(() => {
            this.projectFields();
            this.form?.controls.key.updateValueAndValidity({ emitEvent: false });
        });
        effect(() => {
            const key = this.form?.controls.key;
            if (!key || !this.isNew()) {
                return;
            }
            if (this.isKeyLocked()) {
                key.disable({ emitEvent: false });
            } else {
                key.enable({ emitEvent: false });
            }
        });
    }

    public ngOnInit(): void {
        const source = this.customField();
        this.form = this.fb.group<CustomFieldForm>({
            name: this.fb.control(source.name ?? '', [
                Validators.required,
                Validators.maxLength(60)
            ]),
            key: this.fb.control(source.key ?? '', [
                Validators.required,
                Validators.maxLength(CustomFieldKeyConverter.maxLength),
                Validators.pattern(CustomFieldKeyConverter.pattern),
                control => (this.isKeyTaken(control.value as string) ? { taken: true } : null)
            ]),
            fieldType: this.fb.control<CustomFieldType>(source.fieldType ?? CustomFieldType.Text),
            isRequired: this.fb.control(source.isRequired ?? false)
        });
        this.selectedType.set(this.form.controls.fieldType.value);
        this.options.set([...(source.options ?? [])]);
        this.defaultValue.set(source.defaultValue ?? null);

        if (!this.isNew()) {
            this.form.controls.key.disable();
            this.form.controls.fieldType.disable();
        }

        this.nameValue.set(this.form.controls.name.value);
        this.keyValue.set(this.form.controls.key.value);
        this.isKeyEdited.set(!this.isNew());

        this.form.controls.name.valueChanges.subscribe(name => {
            this.nameValue.set(name);
            if (this.isKeyEdited()) {
                return;
            }
            this.form.controls.key.setValue(CustomFieldKeyConverter.toKey(name));
        });
        this.form.controls.key.valueChanges.subscribe(key => this.keyValue.set(key));

        this.isRequiredChecked.set(this.form.controls.isRequired.value);
        this.form.controls.isRequired.valueChanges.subscribe(isRequired => {
            this.isRequiredChecked.set(isRequired);
            if (!isRequired) {
                this.backfill.set(null);
            }
        });
    }

    protected onKeyInput(): void {
        this.isKeyEdited.set(true);
        // Without this the field would lock again the moment the typed key resolves the
        // clash that opened it.
        this.isKeyRevealed.set(true);
    }

    protected onRevealKey(): void {
        this.isKeyRevealed.set(true);
    }

    private isKeyTaken(key: string): boolean {
        return this.projectFields().some(field => this.isSameKey(field, key));
    }

    private isSameKey(field: CustomField, key: string): boolean {
        return (
            key !== '' &&
            field.key === key &&
            field.idCustomField !== this.customField().idCustomField
        );
    }

    protected onBackfillChange(value: CustomFieldValue): void {
        this.backfill.set(value);
    }

    protected onDefaultChange(value: CustomFieldValue): void {
        this.defaultValue.set(value);
    }

    protected onTypeChange(fieldType: unknown): void {
        this.selectedType.set(fieldType as CustomFieldType);
        this.defaultValue.set(null);
        this.backfill.set(null);
    }

    protected onAddOption(): void {
        this.options.update(options => [
            ...options,
            {
                idOption: 0,
                idCustomField: this.customField().idCustomField ?? 0,
                label: '',
                orderRank: options.length + 1
            }
        ]);
    }

    protected onOptionLabelInput(index: number, event: Event): void {
        this.onOptionLabelChange(index, (event.target as HTMLInputElement).value);
    }

    private onOptionLabelChange(index: number, label: string): void {
        this.options.update(options =>
            options.map((option, i) => (i === index ? { ...option, label } : option))
        );
    }

    protected onRemoveOption(index: number): void {
        this.options.update(options => options.filter((_, i) => i !== index));
    }

    protected get isSaveDisabled(): boolean {
        // The key control is disabled while it follows the name, and a disabled control
        // runs no validators, so its two blockers are checked here instead.
        if (!this.form.valid || !!this.conflictingField() || this.isKeyUngenerated()) {
            return true;
        }
        if (!this.isSelectType()) {
            return false;
        }
        return this.options().length === 0 || this.options().some(option => !option.label.trim());
    }

    protected onSave(): void {
        const source = this.customField();
        const raw = this.form.getRawValue();
        this.save.emit({
            isNew: this.isNew(),
            backfill: this.backfill() ?? undefined,
            field: {
                idCustomField: source.idCustomField ?? 0,
                idProject: source.idProject ?? 0,
                key: raw.key,
                name: raw.name,
                fieldType: raw.fieldType,
                isRequired: raw.isRequired,
                requiredSince: source.requiredSince ?? null,
                defaultValue: this.defaultValue(),
                orderRank: source.orderRank ?? 0,
                archivedAt: source.archivedAt ?? null,
                options: raw.fieldType === CustomFieldType.Select ? this.options() : []
            }
        });
    }

    protected onCancel(): void {
        this.cancelled.emit();
    }
}

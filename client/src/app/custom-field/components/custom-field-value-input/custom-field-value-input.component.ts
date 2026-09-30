import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { CustomFieldType } from '../../constants/custom-field-type.enum';
import { CustomFieldValueConverter } from '../../converter/custom-field-value.converter';
import { CustomFieldOption, CustomFieldValue } from '../../model/custom-field.model';

@Component({
    selector: 'app-custom-field-value-input',
    templateUrl: './custom-field-value-input.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class CustomFieldValueInputComponent {
    public readonly fieldType = input.required<CustomFieldType>();
    public readonly inputId = input.required<string>();
    public readonly options = input<CustomFieldOption[]>([]);
    public readonly value = input<CustomFieldValue>(null);

    public readonly valueChange = output<CustomFieldValue>();

    protected readonly fieldTypes = CustomFieldType;

    protected readonly dateValue = computed(() => CustomFieldValueConverter.toDate(this.value()));

    protected readonly isNumber = computed(() => this.fieldType() === CustomFieldType.Number);

    protected onChange(raw: unknown): void {
        this.valueChange.emit(CustomFieldValueConverter.toValue(this.fieldType(), raw));
    }
}

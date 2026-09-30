import { CustomFieldType } from '../constants/custom-field-type.enum';
import { CustomFieldValue } from '../model/custom-field.model';

export abstract class CustomFieldValueConverter {
    public static toValue(fieldType: CustomFieldType, raw: unknown): CustomFieldValue {
        if (raw === null || raw === undefined || raw === '') {
            return null;
        }
        switch (fieldType) {
            case CustomFieldType.Number:
            case CustomFieldType.Select: {
                const parsed = Number(raw);
                return Number.isNaN(parsed) ? null : parsed;
            }
            case CustomFieldType.Boolean:
                return !!raw;
            case CustomFieldType.Date:
                return raw instanceof Date
                    ? raw.toISOString()
                    : CustomFieldValueConverter.toText(raw);
            default:
                return CustomFieldValueConverter.toText(raw);
        }
    }

    public static toDate(raw: CustomFieldValue): Date | null {
        if (typeof raw !== 'string') {
            return null;
        }
        const parsed = new Date(raw);
        return Number.isNaN(parsed.getTime()) ? null : parsed;
    }

    private static toText(raw: unknown): string | null {
        return typeof raw === 'string' || typeof raw === 'number' ? String(raw) : null;
    }
}

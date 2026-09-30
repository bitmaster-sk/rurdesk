import { CustomFieldType } from '../constants/custom-field-type.enum';

export type CustomFieldValue = string | number | boolean | null;

export interface CustomFieldOption {
    idOption: number;
    idCustomField: number;
    label: string;
    orderRank: number;
}

export interface CustomField {
    idCustomField: number;
    idProject: number;
    key: string;
    name: string;
    fieldType: CustomFieldType;
    isRequired: boolean;
    requiredSince: string | null;
    defaultValue: CustomFieldValue;
    orderRank: number;
    archivedAt: string | null;
    options: CustomFieldOption[];
}

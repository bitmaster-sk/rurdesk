import { OptionConverter } from '../converter/option.converter';

type OptionRecord = Record<string, unknown>;

/**
 * Resolve the bound value of an option. When `optionValue` is set and the option
 * is an object, read that property; otherwise the option itself is the value.
 */
export function resolveOptionValue(option: unknown, optionValue?: string): unknown {
    if (optionValue && option !== null && typeof option === 'object') {
        return (option as OptionRecord)[optionValue];
    }
    return option;
}

/**
 * Resolve the display label of an option. When the option is an object, read
 * `optionLabel` (default `'label'`); otherwise stringify the
 * option. Used only as the fallback when no item template is projected.
 */
export function resolveOptionLabel(option: unknown, optionLabel?: string): string {
    if (option !== null && typeof option === 'object') {
        const key = optionLabel ?? 'label';
        return OptionConverter.toLabel((option as OptionRecord)[key]);
    }
    return OptionConverter.toLabel(option);
}

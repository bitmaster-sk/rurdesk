export abstract class CustomFieldKeyConverter {
    public static readonly maxLength = 40;

    public static readonly pattern = /^[a-z0-9][a-z0-9_]*$/;

    // The key is a jsonb key compared byte for byte, so it stays plain ASCII: the same
    // accented letter has two Unicode spellings and only one of them would ever match.
    public static toKey(name: string): string {
        const ascii = name
            .normalize('NFD')
            .replace(/[\u0300-\u036f]/g, '')
            .toLowerCase()
            .replace(/[^a-z0-9]+/g, '_');
        return CustomFieldKeyConverter.trim(
            CustomFieldKeyConverter.trim(ascii).slice(0, CustomFieldKeyConverter.maxLength)
        );
    }

    private static trim(key: string): string {
        return key.replace(/^_+/, '').replace(/_+$/, '');
    }
}

export abstract class WikiSlugConverter {
    public static readonly maxLength = 120;

    public static toSlug(text: string): string {
        const slug = text
            .normalize('NFD')
            .replace(/\p{Mn}/gu, '')
            .toLowerCase()
            .replace(/[^a-z0-9]+/g, '-')
            .replace(/^-+|-+$/g, '');
        return slug.length > WikiSlugConverter.maxLength
            ? slug.slice(0, WikiSlugConverter.maxLength).replace(/-+$/, '')
            : slug;
    }
}

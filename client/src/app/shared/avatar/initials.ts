export abstract class Initials {
    public static fromName(name: string): string {
        const parts = name.trim().split(' ');
        if (parts.length > 1) {
            return parts
                .slice(0, 2)
                .map(p => p[0])
                .join('')
                .toUpperCase();
        }
        return parts[0].substring(0, 2).toUpperCase();
    }
}

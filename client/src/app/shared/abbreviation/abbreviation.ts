export abstract class Abbreviation {
    /**
     * Two letters for a project tile: the initials of the first two words, or the first two
     * letters of a one-word name, so rurdesk/raycopy (RU/RA) stay apart in the collapsed rail.
     */
    public static of(name: string): string {
        const words = name.split(/[\s&]+/).filter(Boolean);
        if (words.length === 0) {
            return '?';
        }
        const letters =
            words.length > 1
                ? [...words[0]][0] + [...words[1]][0]
                : [...words[0]].slice(0, 2).join('');
        return letters.toLocaleUpperCase();
    }
}

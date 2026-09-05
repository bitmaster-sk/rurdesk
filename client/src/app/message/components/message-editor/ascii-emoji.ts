export abstract class AsciiEmoji {
    // Deliberately no single-letter shortcuts (:b :p :o and their :-x forms). They
    // match Lisp-style keywords and object literals even with the boundary rule
    // below, which is the corruption #91 removed. Do not add them back.
    private static readonly MAP: Record<string, string> = {
        '<3': '💗',
        ':)': '😄',
        ':-)': '😄',
        ':D': '😄',
        ':-D': '😁',
        ';)': '😉',
        ';-)': '😉',
        ':(': '😒',
        ':-(': '😒',
        '%)': '😖',
        '%-)': '😖',
        ':P': '😜',
        ':-P': '😜',
        ':O': '😲',
        ':-O': '😲',
        '>:(': '😠',
        '>:)': '😈',
        '>:-)': '😈',
        '>:/': '😡'
    };

    private static readonly SHORTCUTS = Object.keys(AsciiEmoji.MAP).sort(
        (a, b) => b.length - a.length
    );

    public static matchBeforeCaret(
        textBeforeCaret: string
    ): { shortcut: string; emoji: string } | null {
        if (!textBeforeCaret.endsWith(' ')) {
            return null;
        }
        const upto = textBeforeCaret.slice(0, -1);
        for (const shortcut of AsciiEmoji.SHORTCUTS) {
            if (!upto.endsWith(shortcut)) {
                continue;
            }
            const before = upto[upto.length - shortcut.length - 1];
            if (before === undefined || /\s/.test(before)) {
                return { shortcut, emoji: AsciiEmoji.MAP[shortcut] };
            }
        }
        return null;
    }
}

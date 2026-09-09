export interface EmojiGroup {
    label: string;
    emojis: string[];
}

export const EMOJI_GROUPS: EmojiGroup[] = [
    { label: 'REACTIONS', emojis: ['👍', '👎', '❤️', '🎉', '😂', '😅', '😮', '😢', '😡', '🙏'] },
    { label: 'PEOPLE', emojis: ['😀', '😃', '😄', '😁', '😉', '😊', '😎', '🤔', '😴', '🤯'] },
    { label: 'OBJECTS', emojis: ['✅', '❌', '⭐', '🔥', '💡', '📌', '📎', '🔗', '🔧', '🚀'] },
    { label: 'SYMBOLS', emojis: ['✓', '✗', '→', '←', '↑', '↓', '•', '·', '°', '∞'] }
];

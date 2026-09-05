export abstract class MessageFormatter {
    public static formatUnreadBadgeText(count: number): string | undefined {
        if (count <= 0) return undefined;
        return count > 99 ? '99+' : count.toString();
    }
}

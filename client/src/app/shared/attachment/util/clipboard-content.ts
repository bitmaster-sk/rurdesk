export abstract class ClipboardContent {
    public static isOfficeText(data: DataTransfer | null): boolean {
        return !!data?.types.includes('text/html') && !!data.getData('text/plain');
    }
}

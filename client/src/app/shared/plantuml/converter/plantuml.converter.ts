export abstract class PlantUmlConverter {
    public static toDataUrl(svg: string): string {
        return 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(svg);
    }
}

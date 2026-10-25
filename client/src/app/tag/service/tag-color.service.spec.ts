import { TagColorService } from './tag-color.service';

describe('TagColorService', () => {
    const service = new TagColorService();

    it('offers the first palette color when the project has no tags', () => {
        expect(service.nextColor([])).toBe(service.palette[0]);
    });

    it('skips colors the project already uses, ignoring case', () => {
        const used = [{ color: service.palette[0].toUpperCase() }, { color: service.palette[1] }];

        expect(service.nextColor(used)).toBe(service.palette[2]);
    });

    it('still offers a color once every palette color is taken', () => {
        const used = service.palette.map(color => ({ color }));

        expect(service.palette).toContain(service.nextColor(used));
    });
});

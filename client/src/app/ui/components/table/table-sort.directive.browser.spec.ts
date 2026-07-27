import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { UiModule } from '../../ui.module';
import { UiTableSortEvent } from './table-sort.directive';

@Component({
    standalone: false,
    template: `
        <table
            uiTableSort
            [sortField]="initField"
            [sortOrder]="initOrder"
            (sortChange)="last = $event; emits = emits + 1"
        >
            <thead>
                <tr>
                    <th uiSortColumn="name" #n="uiSortColumn">
                        <i class="ic">{{ n.icon() }}</i>
                    </th>
                    <th uiSortColumn="count" #c="uiSortColumn">
                        <i class="ic">{{ c.icon() }}</i>
                    </th>
                </tr>
            </thead>
            <tbody></tbody>
        </table>
    `
})
class HostComponent {
    public initField: string | null = 'name';
    public initOrder: 1 | -1 = 1;
    public last: UiTableSortEvent | null = null;
    public emits = 0;
}

describe('UiTableSort / UiSortColumn (browser)', () => {
    function setup(configure?: (host: HostComponent) => void) {
        const f = TestBed.createComponent(HostComponent);
        configure?.(f.componentInstance);
        f.detectChanges();
        return f;
    }
    function headers(f: ReturnType<typeof setup>): HTMLElement[] {
        return f.debugElement.queryAll(By.css('th')).map(d => d.nativeElement as HTMLElement);
    }

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [HostComponent],
            imports: [UiModule]
        }).compileComponents();
    });

    it('seeds the initial sorted column without emitting', () => {
        const f = setup();
        const [name, count] = headers(f);
        expect(f.componentInstance.emits).toBe(0);
        expect(name.getAttribute('aria-sort')).toBe('ascending');
        expect(name.querySelector('.ic')?.textContent).toContain('sort-ascending');
        expect(count.getAttribute('aria-sort')).toBe('none');
        expect(count.querySelector('.ic')?.textContent).toContain('arrows-sort');
    });

    it('flips asc↔desc and emits on clicking the sorted column', () => {
        const f = setup();
        const [name] = headers(f);
        name.click();
        f.detectChanges();
        expect(f.componentInstance.last).toEqual({ sortField: 'name', sortOrder: -1 });
        expect(name.getAttribute('aria-sort')).toBe('descending');
        expect(name.querySelector('.ic')?.textContent).toContain('sort-descending');
    });

    it('starts a newly clicked column at ascending and clears the previous one', () => {
        const f = setup();
        const [name, count] = headers(f);
        count.click();
        f.detectChanges();
        expect(f.componentInstance.last).toEqual({ sortField: 'count', sortOrder: 1 });
        expect(count.getAttribute('aria-sort')).toBe('ascending');
        expect(name.getAttribute('aria-sort')).toBe('none');
    });

    it('toggles on Enter/Space keydown', () => {
        const f = setup();
        const [name] = headers(f);
        name.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        f.detectChanges();
        expect(f.componentInstance.emits).toBe(1);
        expect(f.componentInstance.last).toEqual({ sortField: 'name', sortOrder: -1 });
    });

    it('shows no sorted column and does not emit when there is no seed', () => {
        const f = setup(h => (h.initField = null));
        const [name, count] = headers(f);
        expect(f.componentInstance.emits).toBe(0);
        expect(name.getAttribute('aria-sort')).toBe('none');
        expect(count.getAttribute('aria-sort')).toBe('none');
    });

    it('exposes tabindex for keyboard focus on sortable headers', () => {
        const f = setup();
        expect(headers(f).every(h => h.getAttribute('tabindex') === '0')).toBe(true);
    });
});

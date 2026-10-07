import { TestBed } from '@angular/core/testing';
import { TranslateLoader, TranslateModule } from '@ngx-translate/core';
import { Observable, of } from 'rxjs';
import { describe, expect, it } from 'vitest';
import { WikiTableSize } from '../../entity/wiki-table-size.entity';
import { WikiTablePickerComponent } from './wiki-table-picker.component';

class EmptyLoader implements TranslateLoader {
    public getTranslation(): Observable<Record<string, string>> {
        return of({});
    }
}

async function render(): Promise<{
    element: HTMLElement;
    picks: WikiTableSize[];
    detect: () => void;
}> {
    await TestBed.configureTestingModule({
        declarations: [WikiTablePickerComponent],
        imports: [
            TranslateModule.forRoot({ loader: { provide: TranslateLoader, useClass: EmptyLoader } })
        ]
    }).compileComponents();
    const fixture = TestBed.createComponent(WikiTablePickerComponent);
    const picks: WikiTableSize[] = [];
    fixture.componentInstance.picked.subscribe(size => picks.push(size));
    fixture.detectChanges();
    return {
        element: fixture.nativeElement as HTMLElement,
        picks,
        detect: () => fixture.detectChanges()
    };
}

describe('WikiTablePickerComponent', () => {
    it('highlights the hovered size and picks it on click', async () => {
        const { element, picks, detect } = await render();
        const cell = element.querySelector<HTMLElement>('[data-testid="wiki-table-cell-4-2"]')!;

        cell.dispatchEvent(new MouseEvent('mouseenter'));
        detect();

        expect(element.textContent).toContain('4 × 2');
        expect(element.querySelectorAll('.wiki-table-picker__cell--active')).toHaveLength(8);
        cell.click();
        expect(picks).toEqual([{ rows: 4, columns: 2 }]);
    });

    it('can be driven from the keyboard', async () => {
        const { element, picks } = await render();
        const grid = element.querySelector<HTMLElement>('[data-testid="wiki-table-picker"]')!;

        grid.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight' }));
        grid.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown' }));
        grid.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }));

        expect(picks).toEqual([{ rows: 4, columns: 4 }]);
    });
});

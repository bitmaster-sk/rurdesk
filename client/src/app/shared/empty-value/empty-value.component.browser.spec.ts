import { ComponentFixture, TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { By } from '@angular/platform-browser';

import { EmptyValueComponent } from './empty-value.component';
import { EmptyValueAlign } from './constant/empty-value-align.enum';

describe('EmptyValueComponent', () => {
    let fixture: ComponentFixture<EmptyValueComponent>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [EmptyValueComponent],
            imports: [TranslateModule.forRoot()]
        }).compileComponents();
        fixture = TestBed.createComponent(EmptyValueComponent);
    });

    it('shows an em dash as the visible content', () => {
        fixture.detectChanges();

        const dash = fixture.debugElement.query(By.css('.empty-value'));
        expect(dash.nativeElement.textContent.trim()).toBe('—');
    });

    it('hides the dash from assistive tech and exposes a readable label instead', () => {
        fixture.detectChanges();

        const dash = fixture.debugElement.query(By.css('.empty-value'));
        const label = fixture.debugElement.query(By.css('.sr-only'));

        expect(dash.nativeElement.getAttribute('aria-hidden')).toBe('true');
        expect(label.nativeElement.textContent.trim()).toBe('NONE');
    });

    it('indents the dash to the value it replaces', () => {
        // The spacing token lives in the global styles.scss, which the browser test layer does not load.
        document.documentElement.style.setProperty('--ui-space-s', '0.25rem');

        const indentFor = (align: EmptyValueAlign): number => {
            fixture.componentRef.setInput('align', align);
            fixture.detectChanges();
            const dash = fixture.debugElement.query(By.css('.empty-value'));
            return parseFloat(getComputedStyle(dash.nativeElement).paddingLeft);
        };

        expect(indentFor(EmptyValueAlign.Text)).toBe(0);
        expect(indentFor(EmptyValueAlign.Badge)).toBeGreaterThan(indentFor(EmptyValueAlign.Text));
        expect(indentFor(EmptyValueAlign.Dot)).toBeGreaterThan(indentFor(EmptyValueAlign.Badge));
    });
});

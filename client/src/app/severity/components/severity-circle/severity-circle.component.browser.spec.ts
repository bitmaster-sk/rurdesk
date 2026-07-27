import { TestBed } from '@angular/core/testing';
import { SeverityCircleComponent } from './severity-circle.component';

// Proof-of-concept Vitest browser-mode component test: renders the component in a real
// browser via TestBed and asserts the computed inline style.
describe('SeverityCircleComponent (browser)', () => {
    function render(): HTMLElement {
        const fixture = TestBed.createComponent(SeverityCircleComponent);
        return fixture.nativeElement as HTMLElement;
    }

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [SeverityCircleComponent]
        }).compileComponents();
    });

    it('renders a circle with the given background color and size', () => {
        const fixture = TestBed.createComponent(SeverityCircleComponent);
        fixture.componentRef.setInput('color', 'rgb(255, 0, 0)');
        fixture.componentRef.setInput('size', 2);
        fixture.detectChanges();

        const span = fixture.nativeElement.querySelector('span.severity-circle') as HTMLElement;
        expect(span).toBeTruthy();
        expect(span.style.backgroundColor).toBe('rgb(255, 0, 0)');
        expect(span.style.width).toBe('2rem');
        expect(span.style.height).toBe('2rem');
    });

    it('falls back to the surface variable when no color is provided', () => {
        const fixture = TestBed.createComponent(SeverityCircleComponent);
        fixture.detectChanges();

        const span = fixture.nativeElement.querySelector('span.severity-circle') as HTMLElement;
        expect(span.getAttribute('style') ?? '').toContain('--ui-surface-200');
    });
});

import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MermaidDiagramComponent } from './mermaid-diagram.component';
import { MermaidRenderService, MermaidResult } from './mermaid-render.service';

const SOURCE = 'flowchart TD\n    A --> B';
const SVG = '<svg><circle r="4"></circle></svg>';

function renderServiceStub(result: MermaidResult, cached: MermaidResult | null = null) {
    return {
        cached: vi.fn().mockReturnValue(cached),
        render: vi.fn().mockResolvedValue(result)
    };
}

describe('MermaidDiagramComponent (browser)', () => {
    async function setup(stub: ReturnType<typeof renderServiceStub>, debounce = 0) {
        await TestBed.configureTestingModule({
            declarations: [MermaidDiagramComponent],
            imports: [TranslateModule.forRoot()],
            providers: [{ provide: MermaidRenderService, useValue: stub }]
        }).compileComponents();
        const fixture = TestBed.createComponent(MermaidDiagramComponent);
        fixture.componentRef.setInput('source', SOURCE);
        if (debounce > 0) {
            fixture.componentRef.setInput('debounce', debounce);
        }
        return fixture;
    }

    afterEach(() => {
        vi.useRealTimers();
    });

    it('renders without waiting when debounce is 0', async () => {
        const stub = renderServiceStub({ svg: SVG });
        const fixture = await setup(stub);

        fixture.detectChanges();
        expect(stub.render).toHaveBeenCalledTimes(1);

        await vi.waitFor(() => {
            fixture.detectChanges();
            expect(fixture.nativeElement.querySelector('svg')).not.toBeNull();
        });
    });

    it('settles (svg or error) exactly once per render', async () => {
        const stub = renderServiceStub({ svg: SVG });
        const fixture = await setup(stub);
        const settled = vi.fn();
        fixture.componentInstance.settled.subscribe(settled);

        fixture.detectChanges();
        await vi.waitFor(() => expect(settled).toHaveBeenCalledTimes(1));
    });

    it('settles only once the svg is in the DOM', async () => {
        const stub = renderServiceStub({ svg: SVG });
        const fixture = await setup(stub);
        let svgOnSettle: Element | null = null;
        fixture.componentInstance.settled.subscribe(() => {
            svgOnSettle = fixture.nativeElement.querySelector('svg');
        });

        fixture.autoDetectChanges();
        await vi.waitFor(() => expect(svgOnSettle).not.toBeNull());
    });

    it('settles only once the error box is in the DOM', async () => {
        const stub = renderServiceStub({ error: 'Parse error on line 2' });
        const fixture = await setup(stub);
        let errorOnSettle: Element | null = null;
        fixture.componentInstance.settled.subscribe(() => {
            errorOnSettle = fixture.nativeElement.querySelector('[data-testid="mermaid-error"]');
        });

        fixture.autoDetectChanges();
        await vi.waitFor(() => expect(errorOnSettle).not.toBeNull());
    });

    it('keeps a wide diagram at its natural width so the box scrolls instead of shrinking it', async () => {
        const wide =
            '<svg width="100%" style="max-width: 2000px;" viewBox="0 0 2000 100"><rect width="2000" height="100"></rect></svg>';
        const stub = renderServiceStub({ svg: wide });
        const fixture = await setup(stub);
        fixture.nativeElement.style.display = 'block';
        fixture.nativeElement.style.width = '400px';

        fixture.autoDetectChanges();
        await vi.waitFor(() => expect(fixture.nativeElement.querySelector('svg')).not.toBeNull());

        const svg: SVGSVGElement = fixture.nativeElement.querySelector('svg');
        const box: HTMLElement = fixture.nativeElement.querySelector(
            '[data-testid="mermaid-diagram"]'
        );
        expect(svg.getBoundingClientRect().width).toBe(2000);
        expect(box.scrollWidth).toBeGreaterThan(box.clientWidth);
    });

    it('does not stretch a small diagram to the box width', async () => {
        const small =
            '<svg width="100%" style="max-width: 120px;" viewBox="0 0 120 40"><rect width="120" height="40"></rect></svg>';
        const stub = renderServiceStub({ svg: small });
        const fixture = await setup(stub);
        fixture.nativeElement.style.display = 'block';
        fixture.nativeElement.style.width = '600px';

        fixture.autoDetectChanges();
        await vi.waitFor(() => expect(fixture.nativeElement.querySelector('svg')).not.toBeNull());

        expect(fixture.nativeElement.querySelector('svg').getBoundingClientRect().width).toBe(120);
    });

    it('debounces a cache miss by the configured delay', async () => {
        const stub = renderServiceStub({ svg: SVG });
        const fixture = await setup(stub, 300);
        vi.useFakeTimers();
        fixture.detectChanges();

        const box = fixture.nativeElement.querySelector('[data-testid="mermaid-diagram"]');
        expect(box.querySelector('svg')).toBeNull();
        expect(box.textContent).toContain('flowchart TD');
        expect(stub.render).not.toHaveBeenCalled();

        await vi.advanceTimersByTimeAsync(300);
        fixture.detectChanges();

        expect(stub.render).toHaveBeenCalledWith(SOURCE);
        expect(box.querySelector('svg')).not.toBeNull();
    });

    it('shows the error box with the source when the diagram cannot be rendered', async () => {
        const stub = renderServiceStub({ error: 'Parse error on line 2' });
        const fixture = await setup(stub, 300);
        vi.useFakeTimers();
        fixture.detectChanges();

        await vi.advanceTimersByTimeAsync(300);
        fixture.detectChanges();

        const error = fixture.nativeElement.querySelector('[data-testid="mermaid-error"]');
        expect(error).not.toBeNull();
        expect(error.textContent).toContain('flowchart TD');
    });

    it('shows a cached result immediately, without waiting for the debounce', async () => {
        const stub = renderServiceStub({ svg: SVG }, { svg: SVG });
        const fixture = await setup(stub, 300);

        fixture.detectChanges(); // effect applies the cache hit synchronously
        fixture.detectChanges(); // view renders the svg branch

        const box = fixture.nativeElement.querySelector('[data-testid="mermaid-diagram"]');
        expect(box.querySelector('svg')).not.toBeNull();
        expect(stub.render).not.toHaveBeenCalled();
    });
});

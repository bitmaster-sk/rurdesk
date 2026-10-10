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

import { HttpErrorResponse } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { PlantUmlApi } from './plantuml.api.service';
import { PlantUmlDiagramComponent } from './plantuml-diagram.component';

const SOURCE = 'Alice -> Bob: hi';
const SVG = '<svg>diagram</svg>';

describe('PlantUmlDiagramComponent (browser)', () => {
    async function setup(opts: {
        enabled?: boolean;
        response?: { svg: string } | HttpErrorResponse;
        debounce?: number;
    }) {
        const api = {
            render$: vi.fn(() =>
                opts.response instanceof HttpErrorResponse
                    ? throwError(() => opts.response)
                    : of(opts.response ?? { svg: SVG })
            )
        };
        await TestBed.configureTestingModule({
            declarations: [PlantUmlDiagramComponent],
            imports: [TranslateModule.forRoot()],
            providers: [
                { provide: PlantUmlApi, useValue: api },
                {
                    provide: SettingsStore,
                    useValue: { isPlantUmlEnabled: () => opts.enabled ?? true }
                }
            ]
        }).compileComponents();
        const fixture = TestBed.createComponent(PlantUmlDiagramComponent);
        fixture.componentRef.setInput('source', SOURCE);
        if (opts.debounce) {
            fixture.componentRef.setInput('debounce', opts.debounce);
        }
        return { fixture, api };
    }

    afterEach(() => vi.useRealTimers());

    it('renders the svg as an img with a data url', async () => {
        const { fixture } = await setup({});
        fixture.autoDetectChanges();
        await vi.waitFor(() => {
            const img = fixture.nativeElement.querySelector('[data-testid="plantuml-diagram-img"]');
            expect(img).not.toBeNull();
            expect(img.getAttribute('src')).toMatch(/^data:image\/svg\+xml;charset=utf-8,/);
        });
    });

    it('shows the syntax error with the server message and the source', async () => {
        const { fixture } = await setup({
            response: new HttpErrorResponse({ status: 422, error: { error: 'Bad arrow' } })
        });
        fixture.autoDetectChanges();
        await vi.waitFor(() => {
            const box = fixture.nativeElement.querySelector('[data-testid="plantuml-error"]');
            expect(box).not.toBeNull();
            expect(box.textContent).toContain('SHARED.PLANTUML.SYNTAX_ERROR');
            expect(box.textContent).toContain('Bad arrow');
            expect(box.textContent).toContain(SOURCE);
        });
    });

    it('shows the unavailable message for other errors', async () => {
        const { fixture } = await setup({ response: new HttpErrorResponse({ status: 502 }) });
        fixture.autoDetectChanges();
        await vi.waitFor(() => {
            const box = fixture.nativeElement.querySelector('[data-testid="plantuml-error"]');
            expect(box).not.toBeNull();
            expect(box.textContent).toContain('SHARED.PLANTUML.UNAVAILABLE');
        });
    });

    it('debounces the api call and collapses rapid changes into one call', async () => {
        const { fixture, api } = await setup({ debounce: 300 });
        vi.useFakeTimers();
        fixture.detectChanges();
        expect(api.render$).not.toHaveBeenCalled();

        fixture.componentRef.setInput('source', 'A -> C');
        fixture.detectChanges();
        fixture.componentRef.setInput('source', 'A -> D');
        fixture.detectChanges();
        await vi.advanceTimersByTimeAsync(300);
        fixture.detectChanges();

        expect(api.render$).toHaveBeenCalledTimes(1);
        expect(api.render$).toHaveBeenCalledWith('A -> D');
    });

    it('shows the disabled code block and never calls the api while plantuml is off', async () => {
        const { fixture, api } = await setup({ enabled: false });
        const settled = vi.fn();
        fixture.componentInstance.settled.subscribe(settled);
        fixture.autoDetectChanges();

        await vi.waitFor(() => {
            const box = fixture.nativeElement.querySelector('[data-testid="plantuml-disabled"]');
            expect(box).not.toBeNull();
            expect(box.textContent).toContain(SOURCE);
        });
        expect(api.render$).not.toHaveBeenCalled();
        await vi.waitFor(() => expect(settled).toHaveBeenCalledTimes(1));
    });
});

import { TestBed } from '@angular/core/testing';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { MermaidLoaderService } from './mermaid-loader.service';
import { MermaidRenderService, MermaidResult } from './mermaid-render.service';

// The app loads its `--ui-*` tokens from styles.scss; the test DOM has no
// global styles, so provide real values — mermaid's base theme derives
// colors from themeVariables and throws on empty ones.
const TOKENS: Record<string, string> = {
    '--ui-color-primary-weak': '#eef2ff',
    '--ui-color-primary': '#3b5bdb',
    '--ui-color-text': '#1a3263',
    '--ui-color-text-muted': '#64748b',
    '--ui-color-canvas': '#f8fafc',
    '--ui-color-surface': '#ffffff',
    '--ui-color-border': '#e2e8f0',
    '--ui-color-danger': '#dc2626'
};

const svg = (): string => `<svg data-source="${Math.random()}"></svg>`;

// Real MermaidLoaderService → real mermaid 11 with securityLevel 'strict'.
describe('MermaidRenderService (browser, real mermaid)', () => {
    beforeEach(() => {
        for (const [token, value] of Object.entries(TOKENS)) {
            document.documentElement.style.setProperty(token, value);
        }
    });

    afterEach(() => {
        for (const token of Object.keys(TOKENS)) {
            document.documentElement.style.removeProperty(token);
        }
    });

    function service(): MermaidRenderService {
        return TestBed.inject(MermaidRenderService);
    }

    it('renders a flowchart with a hostile label without script or onerror in the svg', async () => {
        const out = await service().render(
            'flowchart TD\n    A["<img src=x onerror=alert(1)>"] --> B[ok]'
        );
        expect('svg' in out).toBe(true);
        if ('svg' in out) {
            expect(out.svg).not.toContain('<script');
            expect(out.svg).not.toContain('onerror');
        }
    });

    it('renders `click … call` without any onclick attribute in strict mode', async () => {
        const out = await service().render('flowchart TD\n    A --> B\n    click A call alert()');
        expect('svg' in out).toBe(true);
        if ('svg' in out) {
            expect(out.svg).not.toContain('onclick');
        }
    });

    it('ignores themeCSS injected through an %%{init}%% directive in the source', async () => {
        const out = await service().render(
            '%%{init: {"themeCSS": "body { background: url(https://x) }"}}%%\n' +
                'flowchart TD\n    A --> B'
        );
        expect('svg' in out).toBe(true);
        if ('svg' in out) {
            expect(out.svg).not.toContain('url(https://x)');
        }
    });

    it('returns an error result for an invalid diagram and leaves no scratch node behind', async () => {
        const out = await service().render('this is not a diagram at all');
        expect('error' in out).toBe(true);
        if ('error' in out) {
            expect(out.error).not.toBe('');
        }
        expect(document.querySelector('[id^="dmermaid-"]')).toBeNull();
    });

    it('answers a rendered source from the cache, synchronously', async () => {
        const source = 'flowchart LR\n    A --> B';
        const first = await service().render(source);
        expect(service().cached(source)).toEqual(first);
        expect(service().cached('never rendered')).toBeNull();
    });
});

describe('MermaidRenderService with a stubbed loader (browser)', () => {
    let load: ReturnType<typeof vi.fn>;

    beforeEach(() => {
        load = vi.fn();
        TestBed.configureTestingModule({
            providers: [{ provide: MermaidLoaderService, useValue: { load } }]
        });
    });

    function service(): MermaidRenderService {
        return TestBed.inject(MermaidRenderService);
    }

    it('recovers after the loader failed once: the first render errors, the second succeeds', async () => {
        load.mockRejectedValueOnce(new Error('import failed')).mockResolvedValue({
            render: vi.fn().mockResolvedValue({ svg: svg() })
        });

        const first = await service().render('flowchart TD\n    A --> B');
        expect('error' in first).toBe(true);
        if ('error' in first) {
            expect(first.error).toContain('import failed');
        }
        // A load failure is transient, so it must not poison the cache.
        expect(service().cached('flowchart TD\n    A --> B')).toBeNull();

        const second = await service().render('flowchart TD\n    A --> B');
        expect('svg' in second).toBe(true);
    });

    it('keeps at most 50 results and evicts the oldest one', async () => {
        load.mockResolvedValue({ render: vi.fn().mockResolvedValue({ svg: svg() }) });
        const rendered: MermaidResult[] = [];
        for (let i = 0; i < 51; i++) {
            rendered.push(await service().render(`flowchart TD\n    A${i} --> B`));
        }
        expect(rendered.every(result => 'svg' in result)).toBe(true);
        expect(service().cached('flowchart TD\n    A0 --> B')).toBeNull();
        expect(service().cached('flowchart TD\n    A1 --> B')).not.toBeNull();
        expect(service().cached('flowchart TD\n    A50 --> B')).not.toBeNull();
    });
});

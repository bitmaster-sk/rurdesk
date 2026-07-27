import { configDefaults, defineConfig } from 'vitest/config';
import { resolve } from 'node:path';

// Bare Vitest setup for the pure-logic unit layer (no Angular builder, no TestBed).
// Specs here must be plain TS — services instantiated directly / mocked.
// Component/DOM tests run under vitest.browser.config.ts (real Chromium +
// TestBed, `*.browser.spec.ts`); Playwright covers end-to-end only.
export default defineConfig({
    resolve: {
        // Project uses absolute `src/...` imports (tsconfig baseUrl: "./").
        alias: {
            src: resolve(__dirname, 'src')
        }
    },
    test: {
        globals: true,
        environment: 'node',
        setupFiles: ['vitest.setup.ts'],
        // All plain `.spec.ts` run here (Karma migration complete — no allow-list).
        // Component/DOM specs use the `.browser.spec.ts` suffix and run under the
        // browser config instead; exclude them so they don't load in node env.
        include: ['src/**/*.spec.ts'],
        exclude: [...configDefaults.exclude, 'src/**/*.browser.spec.ts']
    }
});

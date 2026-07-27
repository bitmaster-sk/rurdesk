import { defineConfig } from 'vitest/config';
import { resolve } from 'node:path';
import angular from '@analogjs/vite-plugin-angular';
import { playwright } from '@vitest/browser-playwright';

// Vitest browser-mode config for isolated Angular component tests (experimental).
// The AnalogJS vite plugin compiles Angular components (inlines templateUrl/styleUrls),
// which bare esbuild cannot do; components render in a real Chromium via Playwright.
// Specs use the `.browser.spec.ts` suffix to keep them separate from the Node unit layer.
export default defineConfig({
    plugins: [angular()],
    resolve: {
        alias: { src: resolve(__dirname, 'src') },
        // Keep a single copy of Angular so injection tokens from external libraries
        // (ngx-markdown, DomSanitizer) resolve against the test injector.
        dedupe: [
            '@angular/core',
            '@angular/common',
            '@angular/platform-browser',
            '@angular/platform-browser-dynamic'
        ]
    },
    // Let the Angular plugin compile these libs instead of esbuild pre-bundling
    // them into a second Angular instance (which breaks inject()).
    optimizeDeps: {
        // Pre-bundle @angular/router in the initial optimize pass. It is imported by
        // only a subset of specs (app, project-builder), so Vite discovers it late
        // and triggers a mid-run re-optimization + reload. That reload splits the
        // module graph, surfacing either as NG0203 ("inject() outside injection
        // context") or as "Vitest failed to find the current suite" (setup file
        // re-evaluated in the wrong context) across unrelated suites. Forcing it
        // here keeps optimization to a single up-front pass.
        include: ['@angular/router'],
        exclude: ['ngx-markdown']
    },
    test: {
        globals: true,
        setupFiles: ['vitest.browser.setup.ts'],
        include: ['src/**/*.browser.spec.ts'],
        browser: {
            enabled: true,
            provider: playwright(),
            headless: true,
            instances: [{ browser: 'chromium' }]
        }
    }
});

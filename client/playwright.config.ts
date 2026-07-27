import { defineConfig, devices } from '@playwright/test';

// Official Playwright E2E config. Tests live in e2e/playwright (kept separate from the
// legacy Protractor specs under e2e/src). The webServer block starts the Angular dev
// server; full agent/issue flows additionally need the API + Postgres + Redis running
// (docker-compose up) — point E2E_BASE_URL at a fully running stack for those.
export default defineConfig({
    testDir: './e2e/playwright',
    fullyParallel: true,
    forbidOnly: !!process.env.CI,
    retries: process.env.CI ? 2 : 0,
    // Terminal summary during the run + HTML report (never auto-open in CI).
    reporter: [['list'], ['html', { open: process.env.CI ? 'never' : 'always' }]],
    use: {
        baseURL: process.env.E2E_BASE_URL ?? 'http://localhost:1000',
        trace: 'on-first-retry'
    },
    projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
    // Locally, start the Angular dev server automatically. When E2E_BASE_URL is
    // set (e.g. CI points it at a full docker-compose stack that also serves the
    // API), skip the client-only dev server — it would not have a backend.
    webServer: process.env.E2E_BASE_URL
        ? undefined
        : {
              command: 'npm start',
              url: 'http://localhost:1000',
              reuseExistingServer: !process.env.CI,
              timeout: 120_000
          }
});

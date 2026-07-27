/**
 * Automated screenshot capture against a locally running stack.
 *
 * Prerequisites:
 *   - stack up (docker compose up) and reachable
 *   - demo data present (run the Kickstart from docs/showcase/demo-scenario.md)
 *   - npm i (playwright is a devDependency) + npx playwright install chromium
 *
 * Usage:
 *   CAPTURE_EMAIL=you@example.com CAPTURE_PASSWORD=secret CAPTURE_PROJECT=1 \
 *     node tools/capture.mjs
 *
 * Env:
 *   CAPTURE_BASE     base URL              (default http://localhost)
 *   CAPTURE_EMAIL    login e-mail          (required)
 *   CAPTURE_PASSWORD login password        (required)
 *   CAPTURE_PROJECT  project id to shoot   (required)
 *   CAPTURE_ISSUE    public issue id for the detail shot (optional)
 *
 * Shots that need live agent runs, dialogs, or a terminal are intentionally
 * NOT here — see site/TASKS.md for the manual list.
 */
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

const IMG = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'assets', 'img');

const BASE = process.env.CAPTURE_BASE || 'http://localhost';
const EMAIL = process.env.CAPTURE_EMAIL;
const PASSWORD = process.env.CAPTURE_PASSWORD;
const PROJECT = process.env.CAPTURE_PROJECT;
const ISSUE = process.env.CAPTURE_ISSUE;

if (!EMAIL || !PASSWORD || !PROJECT) {
    console.error('Set CAPTURE_EMAIL, CAPTURE_PASSWORD and CAPTURE_PROJECT. See header comment.');
    process.exit(1);
}

const SETTLE_MS = 1800; // let charts/gantt/d3 finish rendering

function onLoginPage(page) {
    return new URL(page.url()).pathname.replace(/\/+$/, '').endsWith('/login');
}

async function shoot(page, name, { allowLogin = false } = {}) {
    // Guard against false ✓: if the auth session was lost, the SPA's route guard
    // (auth.guard.ts) bounces every protected route to /login, so without this
    // check we'd silently screenshot the login page and report success. The
    // pre-auth login.png shot opts out via { allowLogin: true }.
    if (!allowLogin && onLoginPage(page)) {
        throw new Error('on /login — session not authenticated (token missing/rejected)');
    }
    await page.waitForTimeout(SETTLE_MS);
    const file = path.join(IMG, name);
    await page.screenshot({ path: file });
    console.log(`✓ ${name}`);
}

// Navigate to a protected route and fail loudly if the guard redirected us back
// to /login (lost session) rather than silently shooting the login page.
async function gotoApp(page, url) {
    await page.goto(url, { waitUntil: 'networkidle' });
    if (onLoginPage(page)) {
        throw new Error(`redirected to /login at ${url} — session lost`);
    }
}

async function tryStep(label, fn) {
    try {
        await fn();
    } catch (err) {
        console.warn(`✗ skipped ${label}: ${err.message.split('\n')[0]}`);
    }
}

const browser = await chromium.launch();
const page = await browser.newPage({
    viewport: { width: 1600, height: 1000 },
    deviceScaleFactor: 2
});

// --- login page (pre-auth shot) ---
await page.goto(`${BASE}/login`, { waitUntil: 'networkidle' });
await shoot(page, 'login.png', { allowLogin: true });

// --- login ---
// Fill the reactive form, submit, and WAIT FOR + VERIFY the result. The previous
// version clicked and `waitForLoadState('networkidle')`, which races the login
// XHR: the script could navigate to a protected route before the token was
// stored, the resolver got a 401, and auth.interceptor.ts wiped the token and
// bounced to /login — leaving every shot on the login page. So: wait for the
// login response, assert it succeeded, and confirm the token landed + we left
// /login before doing anything else.
await page.waitForSelector('[formcontrolname="email"]');
await page.fill('[formcontrolname="email"]', EMAIL);
await page.fill('[formcontrolname="password"]', PASSWORD);

const [loginResponse] = await Promise.all([
    page.waitForResponse((r) => r.url().includes('/api/public/login'), { timeout: 30000 }),
    page.click('button[type="submit"]')
]);
if (!loginResponse.ok()) {
    throw new Error(
        `login failed: POST /api/public/login → ${loginResponse.status()}. ` +
        'Check CAPTURE_EMAIL / CAPTURE_PASSWORD and that the stack is up.'
    );
}
// The SPA stores the token (localStorage 'Authorization') on success, then
// navigates off /login. Wait for both so the route guard won't reject the next
// navigation.
await page.waitForFunction(() => !!localStorage.getItem('Authorization'), null, { timeout: 15000 });
await page.waitForURL((url) => !url.pathname.replace(/\/+$/, '').endsWith('/login'), { timeout: 15000 });
console.log('logged in');

// --- five issue views ---
const views = [
    ['table', 'view-table.png'],
    ['kanban', 'view-kanban.png'],
    ['gantt', 'view-gantt.png'],
    ['calendar', 'view-calendar.png'],
    ['mind', 'view-mind.png']
];
for (const [view, file] of views) {
    await tryStep(file, async () => {
        await gotoApp(page, `${BASE}/project/${PROJECT}/issue/view/${view}`);
        await shoot(page, file);
    });
}

// --- hero: kanban at full glory (retake manually with a live agent run for the money shot) ---
await tryStep('hero.png', async () => {
    await gotoApp(page, `${BASE}/project/${PROJECT}/issue/view/kanban`);
    await shoot(page, 'hero.png');
});

// --- project overview / stats dashboard (issues by state/severity, estimated
// vs tracked, workload by assignee, pinned issues) ---
await tryStep('project-overview.png', async () => {
    await gotoApp(page, `${BASE}/project/${PROJECT}/view`);
    await shoot(page, 'project-overview.png');
});

// --- issue detail ---
await tryStep('issue-detail.png', async () => {
    if (ISSUE) {
        await gotoApp(page, `${BASE}/project/${PROJECT}/issue/${ISSUE}`);
    } else {
        await gotoApp(page, `${BASE}/project/${PROJECT}/issue/view/table`);
        await page.waitForTimeout(SETTLE_MS);
        // Open the first row. Set CAPTURE_ISSUE to target a specific issue if the
        // table markup differs from this selector.
        const row = page.locator('tbody tr a, .p-datatable-tbody tr').first();
        await row.waitFor({ state: 'visible', timeout: 15000 });
        await row.click();
        await page.waitForLoadState('networkidle');
    }
    await shoot(page, 'issue-detail.png');
});

// --- admin user list ---
await tryStep('admin-users.png', async () => {
    await gotoApp(page, `${BASE}/admin/users`);
    await shoot(page, 'admin-users.png');
});

// --- project settings: members + git integrations (tab labels clicked by text) ---
await tryStep('project-members.png', async () => {
    await gotoApp(page, `${BASE}/project/${PROJECT}/settings`);
    const tab = page.getByText(/member/i).first();
    await tab.waitFor({ state: 'visible', timeout: 15000 });
    await tab.click();
    await shoot(page, 'project-members.png');
});

await tryStep('git-integration-settings.png', async () => {
    await gotoApp(page, `${BASE}/project/${PROJECT}/settings`);
    const tab = page.getByText(/git/i).first();
    await tab.waitFor({ state: 'visible', timeout: 15000 });
    await tab.click();
    await shoot(page, 'git-integration-settings.png');
});

await browser.close();
console.log(`\ndone → ${IMG}`);
console.log('remaining manual shots: see site/TASKS.md');

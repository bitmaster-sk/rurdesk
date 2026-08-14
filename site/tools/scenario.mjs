/**
 * End-to-end demo scenario: bootstraps the whole DevToolbox demo (users, bots,
 * teams, project, backlog, sprints, assignments) AND captures every site asset,
 * as an ordered, resumable, step-by-step process.
 *
 *   node tools/scenario.mjs            # interactive TUI (recommended)
 *   node tools/scenario.mjs --list     # print the step list
 *   node tools/scenario.mjs <id>       # run one step by id (e.g. admin-users)
 *
 * Step types:
 *   generate    mutates data (idempotent — skips what already exists)
 *   screenshot  captures a .png; confirm [Enter=ok / r=retry / s=skip]
 *   video       captures a .webm (login/setup trimmed); same confirm
 *   manual      you act in YOUR browser (changes persist → the script sees them),
 *               then confirm in the TUI
 *
 * Two-browser model: this runs its own Playwright session (logged in as the
 * admin). Manual steps are done by YOU in your own Chrome; because they persist
 * to the DB, the Playwright session picks them up on the next step.
 */
import * as L from './capture-lib.mjs';

const PWD = process.env.SCENARIO_PASSWORD || 'kreslo';
const ADMIN = { name: 'John Snow', email: 'john.snow@test.sk', password: PWD };
const PEOPLE = [
  { name: 'Robert Baratheon', email: 'robert.baratheon@test.sk', password: PWD, admin: true, color: '#fda4af' }, // rose (pastel)
  { name: 'Robert Stark', email: 'robert.stark@test.sk', password: PWD, admin: true, color: '#7dd3fc' },     // sky (pastel)
];
const BOTS = [
  { name: 'Qwen', gateway: 'http://gateway-goose-qwen-cloud:9090', color: '#d8b4fe' }, // purple (pastel)
  { name: 'Kimi', gateway: 'http://gateway-goose-kimi-cloud:9090', color: '#fcd34d' }, // amber (pastel)
];
const PROJECT_NAME = process.env.SCENARIO_PROJECT_NAME || 'DevToolbox';

// shared mutable state threaded across steps
const state = { projectId: null, bots: {}, runIssue: process.env.CAPTURE_RUN_ISSUE || null };

// ─────────────────────────── generate helpers (idempotent) ───────────────────────────

async function userRow(page, name) {
  return page.locator('tr', { hasText: name }).first();
}
async function userExists(page, name) {
  await L.gotoApp(page, `${L.BASE}/admin/users`); await L.settle(page);
  return (await page.locator('tr', { hasText: name }).count()) > 0;
}

async function openCreateUser(page) {
  await L.gotoApp(page, `${L.BASE}/admin/users`); await L.settle(page);
  await page.getByRole('button', { name: 'Create user' }).first().click();
  await page.locator('.ui-dialog').first().waitFor({ state: 'visible', timeout: 8000 });
}

// set the create-user avatar colour (#cu-color) so each user gets a distinctive
// colour instead of the app's random default — same input-event dance as #td-color
async function setUserColor(page, hex) {
  if (!hex) return;
  const c = page.locator('#cu-color');
  if (!(await c.count())) return;
  await c.evaluate((el, v) => { el.value = v; el.dispatchEvent(new Event('input', { bubbles: true })); el.dispatchEvent(new Event('change', { bubbles: true })); }, hex);
  await page.waitForTimeout(150);
}

// create a human user (idempotent). If shotFile is set, screenshot the filled form first.
async function ensureUser(page, u, shotFile) {
  if (await userExists(page, u.name)) {
    if (shotFile) { // still capture the form (fill, shoot, cancel — no dup)
      await openCreateUser(page);
      await page.fill('#cu-name', u.name); await page.fill('#cu-email', u.email); await page.fill('#cu-password', u.password);
      await setUserColor(page, u.color);
      await page.waitForTimeout(400);
      await L.shootPage(page, shotFile);
      await page.getByRole('button', { name: 'Cancel' }).first().click().catch(() => page.keyboard.press('Escape'));
    }
    return { created: false };
  }
  await openCreateUser(page);
  await page.fill('#cu-name', u.name); await page.fill('#cu-email', u.email); await page.fill('#cu-password', u.password);
  await setUserColor(page, u.color);
  if (u.admin) await page.locator('#cu-isadmin').check().catch(() => {});
  await page.waitForTimeout(300);
  if (shotFile) await L.shootPage(page, shotFile);
  await page.getByRole('button', { name: 'Create', exact: true }).first().click();
  await page.waitForTimeout(1500);
  return { created: true };
}

// read both tokens from the currently-open bot-keys dialog (visible values)
async function readTokens(page) {
  const text = await page.locator('.ui-dialog:visible').first().innerText().catch(() => '');
  const inputVals = await page.locator('.ui-dialog:visible input').evaluateAll((els) => els.map((e) => e.value).filter(Boolean)).catch(() => []);
  const seen = [], push = (s) => (String(s).match(/[a-f0-9]{40,}/g) || []).forEach((t) => { if (!seen.includes(t)) seen.push(t); });
  push(text); inputVals.forEach(push);
  return seen;
}

// create a bot (idempotent). formShot = screenshot the create form; keysShot =
// screenshot the auto-opened keys dialog WITH VISIBLE tokens (the real result of
// creating a bot). For an existing bot, keys are revealed via Regenerate.
async function ensureBot(page, b, { formShot, keysShot } = {}) {
  const exists = await userExists(page, b.name);
  if (!exists) {
    await openCreateUser(page);
    await page.locator('#cu-isbot').check(); await page.waitForTimeout(400);
    await page.fill('#cu-name', b.name); await page.fill('#cu-gateway-url', b.gateway); await setUserColor(page, b.color); await page.waitForTimeout(300);
    if (formShot) await L.shootPage(page, formShot);
    await page.getByRole('button', { name: 'Create', exact: true }).first().click();
    // keys dialog auto-opens with the freshly-minted, VISIBLE tokens
    await page.getByRole('button', { name: 'Close', exact: true }).first().waitFor({ state: 'visible', timeout: 10000 });
    await page.waitForTimeout(900);
    const seen = await readTokens(page);
    if (keysShot) await L.shootPage(page, keysShot); // dialog on backdrop with visible keys
    return { created: true, gateway: b.gateway, gwToTracker: seen[0] || null, trackerToGw: seen[1] || null, keysOpen: true };
  }
  // exists: optionally shoot the form (fill + cancel), and reveal keys via Regenerate
  if (formShot) {
    await openCreateUser(page); await page.locator('#cu-isbot').check(); await page.waitForTimeout(400);
    await page.fill('#cu-name', b.name); await page.fill('#cu-gateway-url', b.gateway); await setUserColor(page, b.color); await page.waitForTimeout(300);
    await L.shootPage(page, formShot);
    await page.getByRole('button', { name: 'Cancel' }).first().click().catch(() => page.keyboard.press('Escape'));
    await page.waitForTimeout(400);
  }
  let seen = [];
  if (keysShot) {
    await L.gotoApp(page, `${L.BASE}/admin/users`); await L.settle(page);
    await page.locator('tr', { hasText: b.name }).first().locator('button:has(.tabler-icon-key)').first().click();
    await page.locator('.ui-dialog:visible').filter({ hasText: /credentials|token/i }).first().waitFor({ state: 'visible', timeout: 8000 });
    // regenerate both tokens so their values become visible again
    const regen = page.locator('.ui-dialog:visible').getByRole('button', { name: /Regenerate/i });
    const cnt = await regen.count();
    for (let i = 0; i < cnt; i++) { await regen.nth(i).click(); await page.waitForTimeout(900); }
    seen = await readTokens(page);
    await L.shootPage(page, keysShot);
    await page.getByRole('button', { name: 'Close', exact: true }).first().click().catch(() => page.keyboard.press('Escape'));
  }
  return { created: false, gateway: b.gateway, gwToTracker: seen[0] || null, trackerToGw: seen[1] || null };
}

function printEnv(log, name, b) {
  log(`─── gateway env for ${name} (save these — the tokens are shown ONCE) ───`);
  log(`[${name}] GATEWAY_URL=${b.gateway || ''}`);
  if (b.gwToTracker) log(`[${name}] GATEWAY_TO_TRACKER_TOKEN=${b.gwToTracker}`);
  if (b.trackerToGw) log(`[${name}] TRACKER_TO_GATEWAY_TOKEN=${b.trackerToGw}`);
  if (!b.gwToTracker) log(`[${name}] (bot already existed — tokens are shown once; delete+recreate the bot, or Regenerate in its keys dialog)`);
}

async function projectExists(page, name) {
  const r = await L.api(page, 'GET', '/api/private/project');
  if (r.ok && Array.isArray(r.data)) {
    const p = r.data.find((x) => x.name === name);
    return p ? (p.idProject ?? p.id) : null;
  }
  return null;
}

async function getProject(page) {
  if (state.projectId) return state.projectId;
  if (process.env.CAPTURE_PROJECT) return (state.projectId = process.env.CAPTURE_PROJECT);
  const id = await projectExists(page, PROJECT_NAME);
  if (id) return (state.projectId = String(id));
  throw new Error('no project yet — run create-project first (or set CAPTURE_PROJECT)');
}

// Cycle tabs are <div role="button">; only Backlog is a <button> and it carries
// .sprint-tabs__pinned, so excluding that class leaves the real sprints. Closed
// sprints keep a tab but are useless as a rollover target, hence the extra :not.
const OPEN_SPRINT_TABS = '.sprint-tab:not(.sprint-tabs__pinned):not(.sprint-tab--closed)';
const ADD_SPRINT_BUTTON = 'ui-button.sprint-tabs__pinned';
const CHARTS_TOGGLE = '[data-testid="kanban-charts-toggle"] button';
const CHARTS_BAND = '[data-testid="sprint-charts-band"]';

// Creates sprints via the "+ Sprint" dialog until `want` open ones exist. Idempotent
// across re-runs: earlier rollovers close sprints, so a step that needs a live one
// can't assume the seed step's sprints are still open.
async function ensureOpenSprints(page, want) {
  const count = () => page.locator(OPEN_SPRINT_TABS).count();
  for (let guard = 0; guard < want + 2 && (await count()) < want; guard++) {
    const add = page.locator(ADD_SPRINT_BUTTON).first();
    if (!(await add.count())) break;
    await add.scrollIntoViewIfNeeded(); await add.click();
    const d = page.locator('.ui-dialog').first();
    await d.waitFor({ state: 'visible', timeout: 8000 });
    await d.getByRole('button', { name: 'Create', exact: true }).first().click();
    await d.waitFor({ state: 'hidden', timeout: 8000 }).catch(() => {});
    await page.waitForTimeout(800);
  }
  return count();
}

async function getDemoIssue(page, pid) {
  if (state.demoIssue) return state.demoIssue;
  if (process.env.CAPTURE_DEMO_ISSUE) return (state.demoIssue = process.env.CAPTURE_DEMO_ISSUE);
  const r = await L.api(page, 'GET', `/api/private/project/${pid}/issue`);
  const list = Array.isArray(r.data) ? r.data : (r.data?.items ?? r.data?.data ?? []);
  const hit = list.find((i) => (i.idIssuePublic ?? i.publicId ?? i.id));
  if (!hit) throw new Error('no issues in project — run kickstart first');
  return (state.demoIssue = String(hit.idIssuePublic ?? hit.publicId ?? hit.id));
}

// ─────────────────────────────────── steps ───────────────────────────────────
// Each: { id, type, title, file, run(ctx) }. ctx = { page, cursor, mark, log }.

const STEPS = [
  // ── A · Users & access ──
  { id: 'login-shot', type: 'screenshot', file: 'login.png', title: 'Login page (pre-auth)',
    run: async ({ page }) => {
      // the one shot that SHOULD be on /login — log out first, then screenshot directly
      await page.goto(`${L.BASE}/login`, { waitUntil: 'domcontentloaded' });
      await page.evaluate(() => localStorage.removeItem('Authorization')).catch(() => {});
      await page.goto(`${L.BASE}/login`, { waitUntil: 'networkidle' });
      await page.waitForSelector('[formcontrolname="email"]', { timeout: 10000 })
        .catch(() => { throw new Error('login form not found — is the app up? (proxy/client may be returning 502)'); });
      await L.settle(page);
      await page.screenshot({ path: `${L.IMG}/login.png` });
    } },

  { id: 'register', type: 'generate', title: 'Register the first admin (John Snow) — skips if he exists',
    run: async ({ page, log }) => {
      try { await L.loginAs(page, ADMIN.email, ADMIN.password); log('admin already exists — skipped registration'); return; }
      catch { /* fresh stack — register */ }
      await page.goto(`${L.BASE}/register`, { waitUntil: 'networkidle' });
      if (!(await page.locator('[formcontrolname="email"]').count())) throw new Error('no register form at /register — is the app up / is registration still open?');
      await page.fill('[formcontrolname="name"]', ADMIN.name);
      await page.fill('[formcontrolname="email"]', ADMIN.email);
      await page.fill('[formcontrolname="password"]', ADMIN.password);
      await page.fill('[formcontrolname="password2"]', ADMIN.password); // confirm password
      const [res] = await Promise.all([
        page.waitForResponse((r) => r.url().includes('/api/public/register'), { timeout: 20000 }).catch(() => null),
        page.locator('button[type="submit"]').first().click(),
      ]);
      if (res && !res.ok()) throw new Error(`register failed: POST /api/public/register → ${res.status()}`);
      await page.waitForTimeout(1500);
      await L.loginAs(page, ADMIN.email, ADMIN.password);
      log('registered + logged in as admin');
    } },

  { id: 'user-baratheon', type: 'generate', title: 'Create user Robert Baratheon (admin)',
    run: async ({ page, log }) => { const r = await ensureUser(page, PEOPLE[0]); log(r.created ? 'created' : 'already exists — skipped'); } },

  { id: 'create-user-dialog', type: 'screenshot', file: 'create-user-dialog.png', title: 'Create Robert Stark (admin) + shoot the filled form',
    run: async ({ page }) => { await ensureUser(page, PEOPLE[1], 'create-user-dialog.png'); } },

  { id: 'bot-qwen', type: 'generate', title: 'Create bot Qwen (qwen gateway)',
    run: async ({ page, log }) => { const r = await ensureBot(page, BOTS[0]); state.bots.Qwen = r; if (r.keysOpen) await page.getByRole('button', { name: 'Close', exact: true }).first().click().catch(() => {}); printEnv(log, 'Qwen', r); } },

  { id: 'bot-create', type: 'screenshot', file: ['bot-create.png', 'bot-keys.png'], title: 'Create bot Kimi → create form + auto keys dialog (visible tokens) + env dump',
    run: async ({ page, log }) => {
      const r = await ensureBot(page, BOTS[1], { formShot: 'bot-create.png', keysShot: 'bot-keys.png' });
      state.bots.Kimi = r;
      if (r.keysOpen) await page.getByRole('button', { name: 'Close', exact: true }).first().click().catch(() => {});
      printEnv(log, 'Kimi', r);
    } },

  { id: 'teams', type: 'generate', title: 'Teams: Developers (bots) + Managers (people)',
    run: async ({ page, log }) => {
      await L.gotoApp(page, `${L.BASE}/admin/users`); await L.settle(page);
      const panel = page.locator('app-admin-teams');
      const ensureTeam = async (teamName, color) => {
        if (await panel.locator('.team-list', { hasText: teamName }).count()) { log(`${teamName} exists — skipped`); return; }
        await panel.getByRole('button', { name: 'Create team' }).first().click();
        const d = page.locator('.ui-dialog').first(); await d.waitFor({ state: 'visible', timeout: 8000 });
        await d.locator('#td-name').fill(teamName);
        await d.locator('#td-color').evaluate((el, v) => { el.value = v; el.dispatchEvent(new Event('input', { bubbles: true })); el.dispatchEvent(new Event('change', { bubbles: true })); }, color);
        await page.waitForTimeout(300);
        await d.getByRole('button', { name: 'Save', exact: true }).first().click(); await page.waitForTimeout(1000);
        log(`created ${teamName} (${color})`);
      };
      await ensureTeam('Developers', '#a5b4fc'); // indigo (pastel)
      await ensureTeam('Managers', '#6ee7b7');   // emerald (pastel)
      // add members (idempotent): bots → Developers, people → Managers
      const addMembers = async (teamName, names) => {
        await panel.locator('.team-list > div', { hasText: teamName }).first().click(); await page.waitForTimeout(600);
        for (const name of names) {
          if (await page.locator('app-admin-teams tbody tr', { hasText: name }).count()) { log(`${teamName}: ${name} already a member`); continue; }
          await panel.getByRole('button', { name: 'Add member' }).first().click(); await page.waitForTimeout(500);
          const opt = page.locator('.cdk-overlay-container').getByText(name, { exact: true }).first();
          if (!(await opt.count())) { log(`${teamName}: ${name} not available (create the user first) — skipped`); await page.keyboard.press('Escape'); continue; }
          await opt.click(); await page.waitForTimeout(700);
          log(`${teamName}: added ${name}`);
        }
      };
      await addMembers('Developers', ['Kimi', 'Qwen']);
      await addMembers('Managers', ['John Snow', 'Robert Baratheon', 'Robert Stark']);
    } },

  { id: 'admin-users', type: 'screenshot', file: 'admin-users.png', title: 'Admin → users panel (not the teams panel below it)',
    run: async ({ page }) => { await L.gotoApp(page, `${L.BASE}/admin/users`); await L.settle(page); await L.shootElement(page, 'section.panel', 'admin-users.png'); } },

  { id: 'admin-teams', type: 'screenshot', file: 'admin-teams.png', title: 'Admin → teams panel (team selected)',
    run: async ({ page }) => {
      await L.gotoApp(page, `${L.BASE}/admin/users`); await L.settle(page);
      const panel = page.locator('app-admin-teams').first();
      await panel.waitFor({ state: 'visible', timeout: 10000 });
      await panel.locator('.team-list > div').first().click().catch(() => {});
      await page.waitForTimeout(700);
      await panel.screenshot({ path: `${L.IMG}/admin-teams.png` });
    } },

  // ── B · Project + backlog ──
  { id: 'create-project', type: 'generate', title: `Create empty project "${PROJECT_NAME}"`,
    run: async ({ page, log }) => {
      const existing = await projectExists(page, PROJECT_NAME);
      if (existing) { state.projectId = String(existing); log(`exists (id ${existing}) — reused`); return; }
      await L.gotoApp(page, `${L.BASE}/`).catch(() => {});
      await page.locator('button:has(.tabler-icon-plus)').first().click(); await page.waitForTimeout(700);
      await page.fill('#project-title', PROJECT_NAME); await page.waitForTimeout(300);
      const [res] = await Promise.all([
        page.waitForResponse((r) => /\/api\/private\/project\b/.test(r.url()) && r.request().method() === 'POST', { timeout: 15000 }).catch(() => null),
        page.getByRole('button', { name: 'Save', exact: true }).first().click(),
      ]);
      await page.waitForTimeout(1500);
      let id = null; try { id = (await res?.json())?.idProject; } catch { /* */ }
      if (!id) id = await projectExists(page, PROJECT_NAME);
      state.projectId = String(id); log(`created (id ${id})`);
    } },

  { id: 'project-teams', type: 'generate', title: 'Add both teams (Developers + Managers) to the project members',
    run: async ({ page, log }) => {
      const P = await getProject(page);
      await L.gotoApp(page, `${L.BASE}/project/${P}/settings`); await L.settle(page);
      const comp = page.locator('app-project-members').first();
      await comp.waitFor({ state: 'visible', timeout: 15000 });
      for (const team of ['Developers', 'Managers']) {
        if (await comp.locator('tr', { hasText: team }).count()) { log(`${team} already a project member`); continue; }
        await page.getByRole('button', { name: 'Add team' }).first().click(); await page.waitForTimeout(400);
        const opt = page.locator('.cdk-overlay-container').getByText(team, { exact: true }).first();
        if (!(await opt.count())) { log(`${team} not available — skipped`); await page.keyboard.press('Escape'); continue; }
        await opt.click(); await page.waitForTimeout(800);
        log(`added team ${team}`);
      }
    } },

  { id: 'kickstart', type: 'video', file: 'kickstart.webm', title: 'AI Project Builder → backlog (5 tools) → parent Medium → accept → table',
    run: async ({ page, cursor, mark, log }) => {
      const P = await getProject(page);
      const DESC = 'DevToolbox — a single-page web app that hosts a collection of small, independent developer utilities. '
        + 'Each tool is fully self-contained in its own folder under tools/<tool-name>/ with its own UI, logic, and tests, '
        + 'and shares no files with other tools. Build these tools as separate, independent tasks: JSON Formatter & Validator, '
        + 'Base64 Encode/Decode, JWT Decoder, Hash Generator (MD5/SHA-256), Color Converter (HEX/RGB/HSL). '
        + 'Add one parent task "App shell & tool registry" that the individual tools plug into without editing shared code.';
      await L.gotoApp(page, `${L.BASE}/project/${P}/project-builder`); await L.settle(page);
      await cursor.ensure(page); mark(); await page.waitForTimeout(600);
      const boxOf = async (loc) => {
        let b = await loc.first().boundingBox().catch(() => null);
        if (!b) b = await loc.first().locator('ui-select, .ui-select, input, button').first().boundingBox().catch(() => null);
        return b;
      };
      // A missing box used to be swallowed (`if (b)`), which left the cursor frozen
      // wherever it last was — on video that reads as "the cursor vanished under the
      // dropdown". Fail loudly instead; the TUI can retry the step.
      const move = async (loc, what) => {
        const b = await boxOf(loc);
        if (!b) throw new Error(`cursor move failed: no bounding box for ${what} — the clip would show a frozen cursor`);
        await cursor.moveTo(page, Math.round(b.x + b.width / 2), Math.round(b.y + b.height / 2));
      };
      // cursor-aware dropdown pick: glide to the trigger, open it, glide to the
      // option in the CDK overlay, then click it (so the video shows the pointer
      // land on both the box AND the chosen item).
      const pick = async (dropdownLoc, optionText) => {
        await move(dropdownLoc, `dropdown trigger for "${optionText}"`);
        await dropdownLoc.first().click(); await page.waitForTimeout(450);
        const opt = page.locator('.cdk-overlay-container').getByText(optionText, { exact: false }).first();
        await opt.waitFor({ state: 'visible', timeout: 6000 });
        await cursor.ensure(page); // panel just mounted — re-assert the cursor over it
        const b = await opt.boundingBox().catch(() => null);
        if (!b) throw new Error(`cursor move failed: option "${optionText}" has no bounding box`);
        await cursor.moveTo(page, Math.round(b.x + b.width / 2), Math.round(b.y + b.height / 2));
        await page.waitForTimeout(200); await opt.click(); await page.waitForTimeout(400);
      };
      const ta = page.locator('textarea').first();
      await move(ta, 'description textarea'); await ta.click(); await ta.fill(DESC); await page.waitForTimeout(600);
      await pick(page.locator('app-state-dropdown').first(), 'New');
      await pick(page.locator('app-severity-dropdown').first(), 'Low');
      const genBtn = page.getByRole('button', { name: 'Generate Backlog' }).first();
      await move(genBtn, 'Generate Backlog button'); await page.waitForTimeout(300); await genBtn.click();
      const ok = await page.locator('app-staged-issue-tree').first().waitFor({ state: 'visible', timeout: 240000 }).then(() => true).catch(() => false);
      if (!ok) throw new Error('kickstart AI did not return a backlog (flaky — retry, or record this clip manually with Kap)');
      await page.waitForTimeout(1500);
      await pick(page.locator('app-staged-issue-tree app-severity-dropdown').first(), 'Medium');
      const acceptBtn = page.getByRole('button', { name: /Accept All|Accept/ }).first();
      await move(acceptBtn, 'Accept button'); await page.waitForTimeout(300); await acceptBtn.click();
      await page.getByText(/Backlog Created/i).first().waitFor({ state: 'visible', timeout: 60000 }).catch(() => {});
      await page.locator('app-project-builder-step-success button').first().click().catch(() => {});
      await page.waitForTimeout(1500);
      if (!/\/issue\/view/.test(page.url())) await L.gotoApp(page, `${L.BASE}/project/${P}/issue/view/table`);
      await L.settle(page); await page.waitForTimeout(1500);
    } },

  { id: 'quality', type: 'screenshot', file: ['quality-low.png', 'quality-high.png'], title: 'Quality low + high (two throwaway tasks → AI check → shoot → delete)',
    run: async ({ page, log }) => {
      const P = await getProject(page);
      const states = (await L.api(page, 'GET', `/api/private/project/${P}/state`)).data || [];
      const sevs = (await L.api(page, 'GET', `/api/private/project/${P}/severity`)).data || [];
      const idState = states[0]?.idState, idSeverity = sevs[0]?.idSeverity;
      // a human member to assign the high task to → satisfies the Metadata dimension
      const mem = (await L.api(page, 'GET', `/api/private/project/${P}/member`)).data || {};
      const assignee = (mem.users || []).map((u) => u.idUser ?? u.id).find(Boolean) || null;
      const mk = async (title, description, meta = {}) => {
        const r = await L.api(page, 'POST', `/api/private/project/${P}/issue`, { title, description, idState, idSeverity, estimated: 0, tracked: 0, ...meta });
        return r.data?.idIssuePublic ?? r.data?.id;
      };
      const shootQ = async (idIssue, file) => {
        await L.gotoApp(page, `${L.BASE}/project/${P}/issue/${idIssue}`); await L.settle(page);
        const panel = page.locator('.quality-panel').first();
        await panel.waitFor({ state: 'visible', timeout: 10000 });
        // expand so dimensions/problems/suggestions render
        if (await panel.evaluate((el) => el.classList.contains('quality-panel--collapsed')).catch(() => false)) { await page.locator('.quality-panel__header').first().click(); await page.waitForTimeout(500); }
        // run the AI check
        await panel.getByRole('button', { name: /Check Quality|Re-check/ }).first().click();
        // wait for the SCORE BADGE (report done) or an error — not the always-present empty status span
        const done = await L.raceVisible([panel.locator('.quality-badge'), panel.locator('.quality-panel__error')], 90000);
        if (done !== 0) throw new Error('quality check failed/timed out (AI unavailable or rate-limited) — retry');
        await page.waitForTimeout(1300); // let dimensions/problems/suggestions finish rendering
        // isolate + full-page capture so a TALL panel (many problems/suggestions) is
        // shot whole instead of clipped by app-issue-info's scroll; strip border/radius.
        await L.shootElementFull(page, '.quality-panel', file, { stripBorder: true });
      };
      const del = async (id) => { if (id) await L.api(page, 'DELETE', `/api/private/project/${P}/issue/${id}`); };

      // ── low: vague, no metadata → drags every dimension down ──
      let lowId;
      try {
        lowId = await mk('color thing', 'make the color tool work somehow');
        await shootQ(lowId, 'quality-low.png');
      } finally { await del(lowId); }

      // ── high: single-scope, full acceptance criteria, all metadata set
      // (assignee + estimated + state + severity) → maxes all 5 dimensions ──
      const HIGH_DESC = [
        '## Context',
        'Add a self-contained HEX/RGB/HSL color converter to the DevToolbox tool collection. It lives in its own folder and shares no code with other tools, matching the app-shell tool-registry contract.',
        '',
        '## Acceptance criteria',
        '- [ ] `tools/color-converter/` contains the tool UI, logic, and tests; no imports from sibling tools.',
        '- [ ] User enters a color as HEX (`#RRGGBB` / `#RGB` / `#RRGGBBAA`), RGB(A), or HSL(A); the other two formats update live.',
        '- [ ] Invalid input shows an inline validation message and does not update the other fields.',
        '- [ ] A preview swatch reflects the current color, including alpha over a checkerboard background.',
        '- [ ] A "copy" button copies the value of each format to the clipboard.',
        '- [ ] Registered in the tool registry so it appears in the app-shell tool list.',
        '',
        '## Technical notes',
        '- Pure conversion functions (`hexToRgb`, `rgbToHsl`, `hslToRgb`, …) with no DOM access, unit-tested independently.',
        '- Round-trip conversions must be stable (HEX→HSL→HEX returns the original within rounding).',
        '- Import only shared design tokens from `styles.scss`; no cross-tool dependencies.',
        '',
        '## Out of scope',
        'Named-color (CSS keyword) lookup and palette generation — separate tasks.',
        '',
        '## Definition of done',
        'All acceptance criteria met, unit tests green, tool reachable from the app shell.',
      ].join('\n');
      let highId;
      try {
        highId = await mk('Add a HEX/RGB/HSL color converter tool to DevToolbox', HIGH_DESC,
          { assignedTo: assignee, estimated: 480 });
        await shootQ(highId, 'quality-high.png');
      } finally { await del(highId); }
    } },

  { id: 'split', type: 'screenshot', file: ['split-dialog-before-generate.png', 'split-dialog-after-generate.png'], title: 'Split dialog before + after generate (AI, not saved)',
    run: async ({ page }) => {
      const P = await getProject(page); const I = await getDemoIssue(page, P);
      const open = async (hint) => {
        await L.gotoApp(page, `${L.BASE}/project/${P}/issue/${I}`); await L.settle(page);
        await page.locator('app-issue-info button:has(.tabler-icon-settings)').first().click(); await page.waitForTimeout(400);
        await page.getByText('Split', { exact: true }).first().click();
        await page.locator('.ui-dialog').first().waitFor({ state: 'visible', timeout: 8000 });
        if (hint) { await page.locator('.ui-dialog textarea, .ui-dialog input').first().fill(hint); await page.waitForTimeout(300); }
      };
      await open('separate parsing, formatting and error handling');
      await L.shootPage(page, 'split-dialog-before-generate.png');
      await page.getByRole('button', { name: 'Split with AI' }).first().click();
      await page.locator('.children-list').first().waitFor({ state: 'visible', timeout: 120000 });
      await page.waitForTimeout(1000);
      await L.shootPage(page, 'split-dialog-after-generate.png');
      await page.getByRole('button', { name: 'Cancel' }).first().click().catch(() => page.keyboard.press('Escape'));
    } },

  // ── C · Assignments · sprints · views ──
  { id: 'assign', type: 'generate', title: 'Assign open tasks to people (round-robin, not bots)',
    run: async ({ page, log }) => {
      const P = await getProject(page);
      const mem = (await L.api(page, 'GET', `/api/private/project/${P}/member`)).data || {};
      const humans = (mem.users || []).filter((u) => !u.isBot && !(u.email || '').endsWith('bots.local') && (u.idUser ?? u.id));
      if (!humans.length) { log('no human members — run project-teams / create users first'); return; }
      const list = (await L.api(page, 'GET', `/api/private/project/${P}/issue`)).data?.items || [];
      let n = 0, target = 0;
      for (let i = 0; i < list.length; i += 2) { // ~half, deterministic round-robin
        const it = list[i]; const u = humans[(i / 2) % humans.length];
        const pub = it.idIssuePublic ?? it.id; target++;
        // the update endpoint expects the FULL issue object, not a partial patch
        const r = await L.api(page, 'PATCH', `/api/private/project/${P}/issue/${pub}`, { ...it, assignedTo: u.idUser ?? u.id });
        if (r.ok) n++;
      }
      log(`assigned ${n}/${target} tasks across ${humans.length} people`);
    } },

  { id: 'sprints', type: 'generate', title: 'Create sprints (current cycle + a past one)',
    run: async ({ page, log }) => {
      const P = await getProject(page);
      await L.gotoApp(page, `${L.BASE}/project/${P}/issue/view/kanban`); await L.settle(page);
      log(`open sprints now: ${await ensureOpenSprints(page, 2)}`);
    } },

  // ── all hands-on steps in one block: do them back-to-back in YOUR browser,
  // then the rest of the run is unattended (each one persists to the DB).
  { id: 'distribute-sprints', type: 'manual', title: 'Distribute tasks into sprints (drag on the Board)',
    instructions: 'In YOUR browser open the Board (Swimlane) of the DevToolbox project and drag a few tasks\ninto the current sprint + leave some in Backlog. This persists, so the script will see it.' },

  { id: 'adjust-calendar', type: 'manual', title: 'Put some tasks on the calendar (so it isn\'t empty)',
    instructions: 'In YOUR browser open the DevToolbox Calendar and drag a few tasks onto dates\n(or set their scheduled dates) so the calendar shows visible entries. This persists.' },

  { id: 'adjust-gantt', type: 'manual', title: 'Arrange the Gantt nicely (task dates / durations / order)',
    instructions: 'In YOUR browser open the DevToolbox Gantt and tidy the schedule (dates, durations, order)\nso the timeline reads well. This persists, so the script will capture it next.' },

  { id: 'real-git', type: 'manual', title: 'Create a REAL git integration with a real token (list + agent PR need it)',
    instructions: 'In YOUR browser: Project settings → Git integrations → New integration.\nFill a REAL host/repo + a REAL access token and SAVE. (Do NOT paste the token to me.)\nThis persists → the git-integration-settings list screenshot will show it.' },

  { id: 'sprint-board', type: 'screenshot', file: 'sprint-board.png', title: 'Board with sprint tab strip (Swimlane)',
    run: async ({ page }) => { const P = await getProject(page); await L.gotoApp(page, `${L.BASE}/project/${P}/issue/view/kanban`); await L.settle(page); await L.selectSwimlane(page); await L.shootPage(page, 'sprint-board.png'); } },

  { id: 'sprint-charts', type: 'screenshot', file: 'sprint-charts.png', title: 'Charts band above the board (burndown + velocity)',
    run: async ({ page }) => {
      const P = await getProject(page); await L.gotoApp(page, `${L.BASE}/project/${P}/issue/view/kanban`); await L.settle(page); await L.selectSwimlane(page);
      const toggle = page.locator(CHARTS_TOGGLE).first();
      await toggle.waitFor({ state: 'visible', timeout: 10000 });
      if (await toggle.isDisabled()) throw new Error('the Charts toggle is disabled — the project has no cycles');
      if (!(await page.locator(CHARTS_BAND).count())) { await toggle.click(); }
      const band = page.locator(CHARTS_BAND).first();
      await band.waitFor({ state: 'visible', timeout: 10000 });
      await page.waitForTimeout(1200);
      await L.shootPage(page, 'sprint-charts.png');
    } },

  { id: 'sprint-create', type: 'screenshot', file: 'sprint-create.png', title: 'New-sprint dialog (opened, not submitted)',
    run: async ({ page }) => {
      const P = await getProject(page); await L.gotoApp(page, `${L.BASE}/project/${P}/issue/view/kanban`); await L.settle(page); await L.selectSwimlane(page);
      const add = page.locator(ADD_SPRINT_BUTTON).first(); await add.waitFor({ state: 'visible', timeout: 10000 }); await add.scrollIntoViewIfNeeded(); await add.click();
      const d = page.locator('.ui-dialog').first(); await d.waitFor({ state: 'visible', timeout: 8000 }); await page.waitForTimeout(600);
      await L.shootPage(page, 'sprint-create.png');
      // Cancelled on purpose — this step only shoots the form. sprint-rollover seeds
      // the sprints it needs itself rather than depending on a side effect here.
      await page.getByRole('button', { name: 'Cancel' }).first().click().catch(() => page.keyboard.press('Escape'));
    } },

  { id: 'sprint-rollover', type: 'screenshot', file: 'sprint-rollover.png', title: 'Roll over a sprint — carried-over count (Roll over button)',
    run: async ({ page }) => {
      const P = await getProject(page); await L.gotoApp(page, `${L.BASE}/project/${P}/issue/view/kanban`); await L.settle(page); await L.selectSwimlane(page);
      // Self-sufficient: seed the sprints this step needs instead of relying on an
      // earlier one. Roll over needs a *target* — Close moves unfinished issues to the
      // next planned sprint and only falls back to Backlog when there is none, and a
      // Backlog fallback is not what this screenshot is meant to show. So: two open
      // sprints, one to close and one to roll into.
      const open = await ensureOpenSprints(page, 2);
      if (open < 2) throw new Error(`need 2 open sprints to roll between, have ${open}`);
      const openTabs = OPEN_SPRINT_TABS;
      // Roll over is disabled until a cycle sprint is selected — select the one we close
      await page.locator(openTabs).first().click().catch(() => {}); await page.waitForTimeout(600);
      const roll = page.getByRole('button', { name: /Roll over/i }).first();
      await roll.waitFor({ state: 'visible', timeout: 10000 });
      if (await roll.isDisabled().catch(() => false)) throw new Error('Roll over disabled — need a cycle sprint with tasks selected');
      await roll.click(); await page.waitForTimeout(1500);
      // The board reloads unscoped after the close, and the closed sprint leaves the
      // strip — so the first still-open tab is the one that received the tasks. Select
      // it to show them with their ⇄ carry-over count badge.
      await L.selectSwimlane(page).catch(() => {});
      await page.locator(openTabs).first().click().catch(() => {});
      await page.waitForTimeout(900);
      await L.shootPage(page, 'sprint-rollover.png');
    } },

  { id: 'command-palette', type: 'screenshot', file: 'command-palette.png', title: 'Command palette > over a highlighted task (State/Severity)',
    run: async ({ page }) => {
      const P = await getProject(page); await L.gotoApp(page, `${L.BASE}/project/${P}/issue/view/table`); await L.settle(page);
      await page.getByText('Table', { exact: true }).first().click().catch(() => {});
      await page.keyboard.press('ArrowDown'); await page.waitForTimeout(300);
      await page.keyboard.press('ArrowDown'); await page.waitForTimeout(400);
      await page.keyboard.press('Meta+KeyK'); await page.waitForTimeout(600);
      await page.keyboard.type('>set'); await page.waitForTimeout(900);
      await L.shootPage(page, 'command-palette.png'); await page.keyboard.press('Escape');
    } },

  { id: 'view-table', type: 'screenshot', file: 'view-table.png', title: 'Issue table view',
    run: async ({ page }) => { const P = await getProject(page); await L.gotoApp(page, `${L.BASE}/project/${P}/issue/view/table`); await L.settle(page); await L.shootPage(page, 'view-table.png'); } },
  { id: 'quick-actions', type: 'screenshot', file: 'quick-actions.png', title: 'Quick-actions context menu (right-click a table row)',
    run: async ({ page }) => {
      const P = await getProject(page);
      await L.gotoApp(page, `${L.BASE}/project/${P}/issue/view/table`); await L.settle(page);
      const row = page.locator('tbody tr[data-flip-id]').first();
      await row.waitFor({ state: 'visible', timeout: 10000 });
      await row.click({ button: 'right' });
      await page.locator('.qap').waitFor({ state: 'visible', timeout: 10000 });
      await page.waitForTimeout(500);
      await L.shootPage(page, 'quick-actions.png');
      await page.keyboard.press('Escape');
    } },
  { id: 'view-kanban', type: 'screenshot', file: 'view-kanban.png', title: 'Issue kanban view (Swimlane)',
    run: async ({ page }) => { const P = await getProject(page); await L.gotoApp(page, `${L.BASE}/project/${P}/issue/view/kanban`); await L.settle(page); await L.selectSwimlane(page); await L.shootPage(page, 'view-kanban.png'); } },
  { id: 'view-calendar', type: 'screenshot', file: 'view-calendar.png', title: 'Issue calendar view',
    run: async ({ page }) => { const P = await getProject(page); await L.gotoApp(page, `${L.BASE}/project/${P}/issue/view/calendar`); await L.settle(page); await L.shootPage(page, 'view-calendar.png'); } },

  { id: 'view-gantt', type: 'screenshot', file: 'view-gantt.png', title: 'Gantt (Day + Compact)',
    run: async ({ page }) => { const P = await getProject(page); await L.gotoApp(page, `${L.BASE}/project/${P}/issue/view/gantt`); await L.settle(page); await L.selectGanttDayCompact(page); await L.settle(page); await L.shootPage(page, 'view-gantt.png'); } },
  { id: 'hero', type: 'screenshot', file: 'hero.png', title: 'Hero (Gantt Day + Compact)',
    run: async ({ page }) => { const P = await getProject(page); await L.gotoApp(page, `${L.BASE}/project/${P}/issue/view/gantt`); await L.settle(page); await L.selectGanttDayCompact(page); await L.settle(page); await L.shootPage(page, 'hero.png'); } },

  { id: 'gantt-reorder', type: 'video', file: 'gantt-reorder.webm', title: 'Drag a WBS Scheduled row (Day+Compact) — MUTATES order',
    run: async ({ page, cursor, mark }) => {
      const P = await getProject(page);
      await L.gotoApp(page, `${L.BASE}/project/${P}/issue/view/gantt`); await L.settle(page);
      await L.selectGanttDayCompact(page); mark();
      await cursor.ensure(page); await page.waitForTimeout(400);
      const sched = page.getByText('Scheduled', { exact: true }).first();
      if (await sched.count()) { const [sx, sy] = await L.centerOf(sched); await cursor.moveTo(page, sx, sy, 25); await sched.click(); await page.waitForTimeout(700); }
      const rows = page.locator('.wbs-row'); await rows.first().waitFor({ state: 'visible', timeout: 10000 });
      if (await rows.count() < 3) throw new Error('need ≥3 scheduled rows');
      const [srcX, srcY] = await L.centerOf(rows.nth(2)); const dstBox = await rows.nth(0).boundingBox();
      await cursor.moveTo(page, srcX, srcY, 30); await page.waitForTimeout(450);
      await cursor.dragTo(page, srcX, Math.round(dstBox.y + 6), 30); await page.waitForTimeout(900);
      await page.reload({ waitUntil: 'networkidle' }); await L.settle(page);
      await cursor.ensure(page); await page.getByText('Scheduled', { exact: true }).first().click().catch(() => {}); await page.waitForTimeout(1200);
    } },

  { id: 'command-palette-clip', type: 'video', file: 'command-palette.webm', title: 'Palette flow: nav → 2nd task → severity + assignee — MUTATES',
    run: async ({ page, cursor, mark }) => {
      const P = await getProject(page);
      await L.gotoApp(page, `${L.BASE}/project/${P}/issue/view/calendar`); await L.settle(page);
      await cursor.ensure(page); mark(); await page.waitForTimeout(700);
      await page.keyboard.press('/'); await page.waitForTimeout(600); await cursor.ensure(page);
      for (const ch of 'Table') { await page.keyboard.type(ch); await page.waitForTimeout(130); }
      await page.waitForTimeout(700); await page.keyboard.press('Enter'); await L.settle(page); await cursor.ensure(page);
      await page.getByText('Table', { exact: true }).first().click().catch(() => {});
      await page.keyboard.press('ArrowDown'); await page.waitForTimeout(400); await page.keyboard.press('ArrowDown'); await page.waitForTimeout(700);
      await page.keyboard.press('Meta+KeyK'); await page.waitForTimeout(500); await cursor.ensure(page);
      await page.keyboard.type('>'); await page.waitForTimeout(300); for (const ch of 'severity') { await page.keyboard.type(ch); await page.waitForTimeout(120); }
      await page.waitForTimeout(800); await page.keyboard.press('Enter'); await page.waitForTimeout(1000);
      await page.keyboard.press('Meta+KeyK'); await page.waitForTimeout(500); await cursor.ensure(page);
      await page.keyboard.type('>'); await page.waitForTimeout(300); for (const ch of 'assign') { await page.keyboard.type(ch); await page.waitForTimeout(120); }
      await page.waitForTimeout(800); await page.keyboard.press('Enter'); await page.waitForTimeout(1200);
    } },

  { id: 'project-overview', type: 'screenshot', file: 'project-overview.png', title: 'Project overview dashboard',
    run: async ({ page }) => { const P = await getProject(page); await L.gotoApp(page, `${L.BASE}/project/${P}/view`); await L.settle(page); await L.shootPage(page, 'project-overview.png'); } },
  { id: 'mention-picker', type: 'screenshot', file: 'mention-picker.png', title: 'Mention picker autocomplete',
    run: async ({ page }) => {
      const P = await getProject(page); const I = await getDemoIssue(page, P);
      await L.gotoApp(page, `${L.BASE}/project/${P}/issue/${I}`); await L.settle(page);
      const composer = page.locator('.feed-editor').first(); const editor = composer.locator('.editor-input').first();
      await composer.scrollIntoViewIfNeeded(); await editor.click(); await editor.type('@r');
      const picker = page.locator('.mention-picker').first(); await picker.waitFor({ state: 'visible', timeout: 6000 }); await page.waitForTimeout(500);
      const pb = await picker.boundingBox(), cb = await composer.boundingBox();
      const x = Math.min(pb.x, cb.x) - 8, y = Math.min(pb.y, cb.y) - 8;
      await page.screenshot({ path: `${L.IMG}/mention-picker.png`, clip: { x, y, width: Math.max(pb.x + pb.width, cb.x + cb.width) + 8 - x, height: Math.max(pb.y + pb.height, cb.y + cb.height) + 8 - y } });
    } },

  { id: 'mention-chip', type: 'screenshot', file: 'mention-chip.png', title: 'Posted comment with a mention chip — MUTATES',
    run: async ({ page }) => {
      const P = await getProject(page); const I = await getDemoIssue(page, P);
      await L.gotoApp(page, `${L.BASE}/project/${P}/issue/${I}`); await L.settle(page);
      const editor = page.locator('.feed-editor .editor-input').first();
      await editor.scrollIntoViewIfNeeded(); await editor.click();
      await editor.pressSequentially('@Robert', { delay: 12 });
      await page.locator('.mention-picker__item').first().waitFor({ state: 'visible', timeout: 6000 });
      await page.locator('.mention-picker__item').first().click();
      await editor.pressSequentially(' can you review this one?', { delay: 12 }); await page.waitForTimeout(300);
      const item = page.locator('app-activity-comment-item').filter({ has: page.locator('.mention-chip') }).first();
      await page.keyboard.press('Shift+Enter');
      let ok = await item.waitFor({ state: 'visible', timeout: 6000 }).then(() => true).catch(() => false);
      if (!ok) { // fallback: click the Send button
        await page.locator('.feed-editor .editor-toolbar .text-right ui-button').first().click();
        ok = await item.waitFor({ state: 'visible', timeout: 6000 }).then(() => true).catch(() => false);
      }
      if (!ok) throw new Error('mention comment did not post — feed editor send failed');
      await page.waitForTimeout(600);
      await item.scrollIntoViewIfNeeded(); await page.waitForTimeout(300); await item.screenshot({ path: `${L.IMG}/mention-chip.png` });
    } },

  { id: 'participants-panel', type: 'screenshot', file: 'participants-panel.png', title: 'Participants card (assign + comment → 2 people) — MUTATES',
    run: async ({ page }) => {
      const P = await getProject(page); const I = await getDemoIssue(page, P);
      await L.gotoApp(page, `${L.BASE}/project/${P}/issue/${I}`); await L.settle(page);
      await page.locator('app-user-dropdown').first().click(); await page.waitForTimeout(500);
      await page.getByText('Robert Stark', { exact: true }).first().click().catch(() => {}); await page.waitForTimeout(900);
      // post a comment so the author (John) becomes the 2nd participant
      const COMMENT = 'Taking a look — pairing with Robert Stark on this.';
      const editor = page.locator('.feed-editor .editor-input').first();
      await editor.scrollIntoViewIfNeeded(); await editor.click();
      await editor.pressSequentially(COMMENT, { delay: 12 }); await page.waitForTimeout(400);
      const posted = page.locator('app-activity-comment-item').filter({ hasText: 'pairing with Robert Stark' }).first();
      await page.keyboard.press('Shift+Enter');
      let ok = await posted.waitFor({ state: 'visible', timeout: 6000 }).then(() => true).catch(() => false);
      if (!ok) { // fallback: click the Send button in the feed editor toolbar
        await editor.click(); await editor.pressSequentially(COMMENT, { delay: 12 }); await page.waitForTimeout(300);
        await page.locator('.feed-editor .editor-toolbar .text-right ui-button').first().click();
        ok = await posted.waitFor({ state: 'visible', timeout: 6000 }).then(() => true).catch(() => false);
      }
      if (!ok) throw new Error('comment did not post — feed editor send failed (only the assignee would show as participant)');
      await page.waitForTimeout(900);
      const root = page.locator('app-issue-participants .participants-panel').first(); await root.waitFor({ state: 'visible', timeout: 8000 });
      if (await page.locator('app-issue-participants .participants-panel--collapsed').count()) { await page.locator('app-issue-participants .participants-panel__header').first().click(); await page.waitForTimeout(600); }
      await root.scrollIntoViewIfNeeded(); await page.waitForTimeout(400); await root.screenshot({ path: `${L.IMG}/participants-panel.png` });
    } },

  { id: 'tracker', type: 'screenshot', file: 'tracker.png', title: 'Feed → Time chip: two users\' tracked time — MUTATES',
    run: async ({ page }) => {
      const P = await getProject(page); const I = await getDemoIssue(page, P);
      const logTime = async (value) => {
        await L.gotoApp(page, `${L.BASE}/project/${P}/issue/${I}`); await L.settle(page);
        const input = page.locator('app-tracker input[uiInput], app-tracker input[type="text"]').first();
        await input.scrollIntoViewIfNeeded(); await input.click(); await input.fill(value); await input.press('Enter').catch(() => {});
        await page.locator('app-tracker').first().locator('button').first().click().catch(() => {}); await page.waitForTimeout(1200);
      };
      await logTime('45m');
      await L.loginAs(page, PEOPLE[1].email, PEOPLE[1].password); await logTime('1h 15m');
      await L.gotoApp(page, `${L.BASE}/project/${P}/issue/${I}`); await L.settle(page);
      const feed = page.locator('app-issue-activity-feed').first();
      await feed.locator('.feed-header .chip').filter({ hasText: 'Time' }).first().click(); await page.waitForTimeout(800);
      await feed.scrollIntoViewIfNeeded(); await page.waitForTimeout(400); await feed.screenshot({ path: `${L.IMG}/tracker.png` });
      await L.loginAs(page, ADMIN.email, ADMIN.password);
    } },

  // shot LAST of the detail-page group so the feed already shows comments + tracked time
  { id: 'issue-detail', type: 'screenshot', file: 'issue-detail.png', title: 'Issue detail (feed filled: comments + time)',
    run: async ({ page }) => { const P = await getProject(page); const I = await getDemoIssue(page, P); await L.gotoApp(page, `${L.BASE}/project/${P}/issue/${I}`); await L.settle(page); await L.shootPage(page, 'issue-detail.png'); } },

  { id: 'project-members', type: 'screenshot', file: 'project-members.png', title: 'Project settings → members (users + teams)',
    run: async ({ page }) => {
      const P = await getProject(page); await L.gotoApp(page, `${L.BASE}/project/${P}/settings`); await L.settle(page);
      const comp = page.locator('app-project-members').first(); await comp.waitFor({ state: 'visible', timeout: 15000 });
      await comp.scrollIntoViewIfNeeded(); await page.waitForTimeout(500); await comp.screenshot({ path: `${L.IMG}/project-members.png` });
    } },

  { id: 'git-integration-settings', type: 'screenshot', file: 'git-integration-settings.png', title: 'Project settings → git integrations (list)',
    run: async ({ page }) => { const P = await getProject(page); await L.gotoApp(page, `${L.BASE}/project/${P}/settings`); const t = page.getByText(/git/i).first(); await t.waitFor({ state: 'visible', timeout: 15000 }); await t.click(); await L.settle(page); await L.shootPage(page, 'git-integration-settings.png'); } },

  { id: 'git-integration-form', type: 'screenshot', file: 'git-integration-form.png', title: 'Git integration create form (filled, cancelled)',
    run: async ({ page }) => {
      const P = await getProject(page); await L.gotoApp(page, `${L.BASE}/project/${P}/settings`); await L.settle(page);
      const b = page.getByRole('button', { name: 'New integration' }).first(); await b.waitFor({ state: 'visible', timeout: 15000 }); await b.scrollIntoViewIfNeeded(); await b.click();
      const d = page.locator('.ui-dialog').first(); await d.waitFor({ state: 'visible', timeout: 8000 });
      await page.fill('#gi-name', 'DevToolbox · GitHub'); await page.fill('#gi-base-url', 'https://github.com'); await page.fill('#gi-repo-path', 'acme/devtoolbox'); await page.fill('#gi-token', 'ghp_exampleTokenNotReal0000000000000000'); await page.waitForTimeout(500);
      await L.shootPage(page, 'git-integration-form.png');
      await page.getByRole('button', { name: 'Cancel' }).first().click().catch(() => page.keyboard.press('Escape'));
    } },

  // ── D · Agent run ──
  { id: 'agent-run', type: 'manual', title: 'Drive an agent run to "wait for merge" (pr_open)',
    instructions: 'In YOUR browser: assign an issue to a bot, approve the gated stages, let it reach pr_open.\nThen enter that issue\'s public id below (it feeds the agent screenshots).',
    run: async () => { const id = await L.promptLine('   run issue public id → '); if (id) state.runIssue = L.parseIssueRef(id); } },

  { id: 'stage-brainstorming', type: 'screenshot', file: 'stage-brainstorming.png', title: 'Brainstorming stage comment',
    run: async ({ page }) => { const P = await getProject(page); await L.gotoApp(page, `${L.BASE}/project/${P}/issue/${state.runIssue}`); await L.settle(page); let el = page.locator('.comment-card.kind-brainstorming-complete').first(); if (!(await el.count())) el = page.locator('.comment-card.kind-brainstorming-question').first(); await el.waitFor({ state: 'visible', timeout: 12000 }); await el.scrollIntoViewIfNeeded(); await page.waitForTimeout(400); await el.screenshot({ path: `${L.IMG}/stage-brainstorming.png` }); } },
  { id: 'stage-design', type: 'screenshot', file: 'stage-design.png', title: 'Design stage comment',
    run: async ({ page }) => { const P = await getProject(page); await L.gotoApp(page, `${L.BASE}/project/${P}/issue/${state.runIssue}`); await L.settle(page); await L.shootCommentTop(page, '.comment-card.kind-design', 'stage-design.png'); } },
  { id: 'stage-plan', type: 'screenshot', file: 'stage-plan.png', title: 'Implementation plan stage comment',
    run: async ({ page }) => { const P = await getProject(page); await L.gotoApp(page, `${L.BASE}/project/${P}/issue/${state.runIssue}`); await L.settle(page); await L.shootCommentTop(page, '.comment-card.kind-implementation-plan', 'stage-plan.png'); } },
  { id: 'message-mockup', type: 'screenshot', file: 'message-mockup.png', title: 'Design comment mockup cards (one selected) + surroundings',
    run: async ({ page }) => {
      const P = await getProject(page); await L.gotoApp(page, `${L.BASE}/project/${P}/issue/${state.runIssue}`); await L.settle(page);
      // target the design comment that ACTUALLY carries mockups (a run can have
      // several design comments; only one holds the cards). The mockup cards are
      // small collapsed buttons at the very bottom of a long comment, so shoot the
      // tail — both mockups + a slice of design context — not the whole prose.
      await L.shootCommentMockups(page, '.comment-card.kind-design:has(app-mockup-card)', 'message-mockup.png');
    } },
  { id: 'stage-mockup-preview', type: 'screenshot', file: 'stage-mockup-preview.png', title: 'Opened sandboxed mockup preview',
    run: async ({ page }) => {
      const P = await getProject(page); await L.gotoApp(page, `${L.BASE}/project/${P}/issue/${state.runIssue}`); await L.settle(page);
      const card = page.locator('.mockup-card').first(); await card.waitFor({ state: 'visible', timeout: 12000 }); await card.scrollIntoViewIfNeeded(); await card.click();
      const modal = page.locator('.ui-dialog').filter({ has: page.locator('.mockup-frame-wrap') }).first(); await modal.waitFor({ state: 'visible', timeout: 8000 }); await page.waitForTimeout(1200);
      await L.shootPage(page, 'stage-mockup-preview.png'); await page.keyboard.press('Escape');
    } },
  { id: 'run-pr', type: 'screenshot', file: 'run-pr.png', title: 'PR-pushed comment with the PR link',
    run: async ({ page }) => { const P = await getProject(page); await L.gotoApp(page, `${L.BASE}/project/${P}/issue/${state.runIssue}`); await L.settle(page); await L.shootElement(page, '.comment-card.kind-pull-request-pushed', 'run-pr.png'); } },
  { id: 'run-card', type: 'screenshot', file: 'run-card.png', title: 'Agent run card (stage timeline + phase badge)',
    run: async ({ page }) => {
      const P = await getProject(page); await L.gotoApp(page, `${L.BASE}/project/${P}/issue/${state.runIssue}`); await L.settle(page);
      const card = page.locator('.agent-run-card').first(); await card.waitFor({ state: 'visible', timeout: 12000 });
      if (!(await card.evaluate((el) => el.classList.contains('is-open')).catch(() => true))) { await card.locator('.card-summary').first().click().catch(() => {}); await page.waitForTimeout(600); }
      await card.scrollIntoViewIfNeeded(); await page.waitForTimeout(400); await card.screenshot({ path: `${L.IMG}/run-card.png` });
    } },
  { id: 'mr-diff', type: 'screenshot', file: 'mr-diff.png', title: 'Change request panel diffs (wait-for-merge)',
    run: async ({ page }) => {
      const P = await getProject(page);
      if (!state.runIssue) throw new Error('no run issue set — run the `agent-run` step first (enter the pr_open issue id) or set CAPTURE_RUN_ISSUE');
      await L.gotoApp(page, `${L.BASE}/project/${P}/issue/${state.runIssue}`); await L.settle(page);
      // the pr-panel renders for EVERY saved issue; if it's missing the issue didn't
      // load (bad/blank runIssue → isNewIssue) — surface that instead of a bare timeout.
      const info = page.locator('app-issue-info').first();
      await info.waitFor({ state: 'visible', timeout: 12000 }).catch(() => { throw new Error(`issue ${state.runIssue} did not load — check the run issue id`); });
      // scope to app-issue-info — the participants panel ALSO uses `.pr-panel`, so
      // an unscoped selector grabs the wrong panel. This is the MR/change-request panel.
      const panel = page.locator('app-issue-info .pr-panel').first();
      await panel.waitFor({ state: 'visible', timeout: 12000 });
      if (await page.locator('app-issue-info .pr-panel--collapsed').count()) { await page.locator('app-issue-info .pr-panel__header').first().click(); await page.waitForTimeout(600); }
      // the diff-viewer has NO vertical cap (only overflow-x), so the panel body grows
      // to the FULL diff height — a raw element screenshot is then a giant ribbon where
      // the header is a sliver. Clamp the body to a bounded, scrollable window so the
      // whole panel (header + a readable diff slice) shoots at normal size.
      await page.evaluate(() => {
        const body = document.querySelector('app-issue-info .pr-panel__body');
        if (body) { body.style.maxHeight = '520px'; body.style.overflowY = 'auto'; }
      });
      await page.waitForTimeout(300);
      await L.shootElement(page, 'app-issue-info .pr-panel', 'mr-diff.png');
    } },

  { id: 'videos-manual', type: 'manual', title: 'Record the remaining clips with Kap',
    instructions: 'These need live recording (Kap → export .webm into site/assets/img/):\n  • agent-workflow.webm — the whole run\n  • relation-drag.webm — Gantt connection-handle drag + cascade' },
];

// ─────────────────────────────────── runner ───────────────────────────────────

function stepLabel(s, i) {
  const t = { generate: '⚙', screenshot: '📸', video: '🎬', manual: '✋' }[s.type] || '·';
  const st = { ok: '✓', failed: '✗', skipped: '−' }[s._status] || ' ';
  const f = s.file ? `  (${s.file})` : '';
  return `${String(i + 1).padStart(2)}. [${st}] ${t} ${s.id} — ${s.title}${f}`;
}

async function ensureAdmin(page) {
  const tok = await page.evaluate(() => localStorage.getItem('Authorization')).catch(() => null);
  if (!tok || L.onLoginPage(page)) { try { await L.loginAs(page, ADMIN.email, ADMIN.password); } catch { /* fresh stack: register step handles it */ } }
}

async function runStep(step, shared) {
  const log = (m) => console.log(`    ${m}`);
  if (step.type === 'manual') {
    console.log(`\n✋ MANUAL — ${step.title}`);
    if (step.instructions) console.log(step.instructions.split('\n').map((l) => '   ' + l).join('\n'));
    const a = (await L.promptLine('   done? press Enter (or type s to skip) → ')).trim().toLowerCase();
    step._status = a === 's' ? 'skipped' : 'ok';
    if (step._status === 'ok' && step.run) await step.run({ page: shared.page, state, log });
    return;
  }
  if (step.type === 'video') {
    const { browser, context, page } = await L.launch({ video: true });
    const cursor = L.cursorState(); const t0 = Date.now(); let markMs = null;
    const mark = () => { if (markMs === null) markMs = Date.now(); };
    try { await L.loginAs(page, ADMIN.email, ADMIN.password); await step.run({ page, cursor, mark, log, state }); }
    finally { await context.close(); await browser.close(); }
    const offset = markMs ? Math.max(0, (markMs - t0) / 1000 - 0.4) : 0;
    L.finalizeWebm(step.file, offset);
    return;
  }
  // generate / screenshot use the shared admin session
  await ensureAdmin(shared.page);
  await step.run({ page: shared.page, cursor: shared.cursor, log, state });
}

async function execWithConfirm(step, shared, auto) {
  while (true) {
    step._status = 'pending';
    let error = null;
    try { await runStep(step, shared); } catch (e) { error = e; }
    if (step.type === 'manual') return true; // status set inside
    if (error) {
      step._status = 'failed';
      console.log(`  ✗ ${step.id}: ${String(error.message).split('\n')[0]}`);
      const a = (await L.promptLine('  [r=retry / s=skip / q=quit] → ')).trim().toLowerCase();
      if (a === 'r') continue; if (a === 'q') return false; step._status = 'skipped'; return true;
    }
    if (step.type === 'generate') { step._status = step._status === 'skipped' ? 'skipped' : 'ok'; console.log(`  ✓ ${step.id}`); return true; }
    // screenshot / video → confirm
    step._status = 'ok';
    const files = Array.isArray(step.file) ? step.file : [step.file];
    files.forEach((f) => console.log(`  ✓ ${step.id} → ${L.IMG}/${f}`));
    if (auto) return true;
    const a = (await L.promptLine('  [Enter=ok / r=retry / s=skip] → ')).trim().toLowerCase();
    if (a === 'r') continue; if (a === 's') { step._status = 'skipped'; }
    return true;
  }
}

async function main() {
  const argv = process.argv.slice(2);
  if (argv.includes('--list')) { STEPS.forEach((s, i) => console.log(stepLabel(s, i))); return; }

  const { browser, context, page } = await L.launch();
  const shared = { page, cursor: L.cursorState() };
  try {
    if (argv.length && !argv[0].startsWith('--')) {
      const step = STEPS.find((s) => s.id === argv[0]);
      if (!step) { console.error(`unknown step: ${argv[0]} (see --list)`); return; }
      await execWithConfirm(step, shared, false);
      return;
    }
    // interactive TUI
    let lastRun = -1;
    while (true) {
      console.log('\n=== DevToolbox demo scenario ===');
      STEPS.forEach((s, i) => console.log(stepLabel(s, i)));
      const cmd = (await L.promptLine('\nstep # · n next · "a N" all-from-N · "N-M" range (auto) · q quit → ')).trim();
      if (cmd === 'q' || cmd === '') break;
      let start, end, auto = false;
      const range = cmd.match(/^(\d+)\s*-\s*(\d+)$/);
      if (cmd === 'n') { start = lastRun + 1; end = start + 1; }
      else if (cmd.startsWith('a')) { auto = true; start = (parseInt(cmd.slice(1).trim(), 10) || 1) - 1; end = STEPS.length; }
      else if (range) { auto = true; start = parseInt(range[1], 10) - 1; end = parseInt(range[2], 10); }
      else { start = (parseInt(cmd, 10) || 0) - 1; end = start + 1; }
      if (start < 0 || start >= STEPS.length) { console.log(start >= STEPS.length ? 'already at the last step' : 'out of range'); continue; }
      end = Math.min(end, STEPS.length);
      let cont = true;
      for (let i = start; i < end && cont; i++) { console.log(`\n▶ ${stepLabel(STEPS[i], i)}`); cont = await execWithConfirm(STEPS[i], shared, auto); lastRun = i; }
    }
  } finally { await context.close(); await browser.close(); }
}

main();

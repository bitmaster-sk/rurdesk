/**
 * Interactive capture harness — pick screenshots/clips from a checkbox menu,
 * they run against the live stack and save straight into site/assets/img/.
 *
 * USAGE
 *   node tools/capture-menu.mjs                 # interactive checkbox menu (TTY)
 *   node tools/capture-menu.mjs --list          # list every available shot id
 *   node tools/capture-menu.mjs --all           # run every shot, no prompt
 *   node tools/capture-menu.mjs view-table command-palette   # run specific ids
 *   node tools/capture-menu.mjs --png           # run all png shots
 *   node tools/capture-menu.mjs --webm          # run all video shots
 *
 * ENV (defaults target the demo stack from site/TASKS.md)
 *   CAPTURE_BASE      base URL              (default http://localhost)
 *   CAPTURE_EMAIL     login e-mail          (default john.snow@test.sk)
 *   CAPTURE_PASSWORD  login password        (default kreslo)
 *   CAPTURE_PROJECT   project id            (default 1)
 *   CAPTURE_ISSUE     public issue id for the detail shot (optional)
 *
 * NOT INCLUDED (do these by hand — they need real AI runs or mutate data in
 * ways better done manually): Project Kickstart / AI import, the agent-run
 * "money shots".
 */
import path from 'node:path';
import fs from 'node:fs';
import os from 'node:os';
import readline from 'node:readline';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const IMG = path.resolve(HERE, '..', 'assets', 'img');

const BASE = (process.env.CAPTURE_BASE || 'http://localhost').replace(/\/+$/, '');
const EMAIL = process.env.CAPTURE_EMAIL || 'john.snow@test.sk';
const PASSWORD = process.env.CAPTURE_PASSWORD || 'kreslo';
const PROJECT = process.env.CAPTURE_PROJECT || '1';
const ISSUE = process.env.CAPTURE_ISSUE || '';
// second (human) user for participant/tracker shots that need two people
const EMAIL2 = process.env.CAPTURE_EMAIL2 || 'robert.stark@test.sk';
const PASSWORD2 = process.env.CAPTURE_PASSWORD2 || 'kreslo';
const USER2_NAME = process.env.CAPTURE_USER2_NAME || 'Robert Stark';
// demo issue (public id) the mutating shots act on; split can target its own
const DEMO_ISSUE = process.env.CAPTURE_DEMO_ISSUE || '20';
const SPLIT_ISSUE = process.env.CAPTURE_SPLIT_ISSUE || DEMO_ISSUE;

const VW = 1600, VH = 1000;
const SETTLE = 1800; // let charts/gantt/d3 settle

// ─────────────────────────── shared browser helpers ───────────────────────────

function onLoginPage(page) {
  return new URL(page.url()).pathname.replace(/\/+$/, '').endsWith('/login');
}

async function login(page) {
  await page.goto(`${BASE}/login`, { waitUntil: 'networkidle' });
  await page.waitForSelector('[formcontrolname="email"]');
  await page.fill('[formcontrolname="email"]', EMAIL);
  await page.fill('[formcontrolname="password"]', PASSWORD);
  const [res] = await Promise.all([
    page.waitForResponse((r) => r.url().includes('/api/public/login'), { timeout: 30000 }),
    page.click('button[type="submit"]'),
  ]);
  if (!res.ok()) throw new Error(`login failed: POST /api/public/login → ${res.status()} (check CAPTURE_EMAIL/PASSWORD, stack up?)`);
  await page.waitForFunction(() => !!localStorage.getItem('Authorization'), null, { timeout: 15000 });
  await page.waitForURL((u) => !u.pathname.replace(/\/+$/, '').endsWith('/login'), { timeout: 15000 });
}

// log in as a specific user, replacing any current session (for multi-user shots)
async function loginAs(page, email, password) {
  await page.goto(`${BASE}/login`, { waitUntil: 'domcontentloaded' });
  await page.evaluate(() => localStorage.removeItem('Authorization'));
  await page.goto(`${BASE}/login`, { waitUntil: 'networkidle' });
  await page.waitForSelector('[formcontrolname="email"]');
  await page.fill('[formcontrolname="email"]', email);
  await page.fill('[formcontrolname="password"]', password);
  const [res] = await Promise.all([
    page.waitForResponse((r) => r.url().includes('/api/public/login'), { timeout: 30000 }),
    page.click('button[type="submit"]'),
  ]);
  if (!res.ok()) throw new Error(`login failed for ${email}: ${res.status()}`);
  await page.waitForFunction(() => !!localStorage.getItem('Authorization'), null, { timeout: 15000 });
  await page.waitForURL((u) => !u.pathname.replace(/\/+$/, '').endsWith('/login'), { timeout: 15000 });
}

async function gotoApp(page, url) {
  await page.goto(url, { waitUntil: 'networkidle' });
  if (onLoginPage(page)) throw new Error(`redirected to /login at ${url} — session lost`);
}

async function settle(page, ms = SETTLE) { await page.waitForTimeout(ms); }

// switch the Board layout toggle (Columns | Swimlane) to Swimlane
async function selectSwimlane(page) {
  const btn = page.getByText('Swimlane', { exact: true }).first();
  if (await btn.count()) { await btn.click(); await page.waitForTimeout(800); }
}

// set the Gantt toolbar to Day zoom + Compact density (exact text avoids "Quarter Day")
async function selectGanttDayCompact(page) {
  for (const label of ['Day', 'Compact']) {
    const btn = page.getByText(label, { exact: true }).first();
    if (await btn.count()) { await btn.click(); await page.waitForTimeout(700); }
  }
}

// locate playwright's bundled ffmpeg (macOS) for trimming clips
function findFfmpeg() {
  const base = path.join(os.homedir(), 'Library', 'Caches', 'ms-playwright');
  try {
    for (const d of fs.readdirSync(base)) {
      if (!d.startsWith('ffmpeg-')) continue;
      const p = path.join(base, d, 'ffmpeg-mac');
      if (fs.existsSync(p)) return p;
    }
  } catch { /* no cache dir */ }
  return null;
}

// re-encode a webm starting at `offsetSec` (drops the login + setup lead-in)
function trimWebm(file, offsetSec) {
  const ff = findFfmpeg();
  if (!ff || offsetSec <= 0.05) return false;
  const tmp = file.replace(/\.webm$/, '.trim.webm');
  execFileSync(ff, ['-y', '-ss', offsetSec.toFixed(2), '-i', file, '-c:v', 'libvpx', '-b:v', '1500k', '-an', tmp], { stdio: 'ignore' });
  fs.renameSync(tmp, file);
  return true;
}

async function shoot(page, name) {
  if (onLoginPage(page)) throw new Error('on /login — session not authenticated');
  await page.screenshot({ path: path.join(IMG, name) });
}

// ── injected, movable cursor (Playwright video omits the OS pointer) ──
const CURSOR_JS = `(() => {
  if (window.__cur) return;
  const c = document.createElement('div');
  c.id = '__fakecursor';
  Object.assign(c.style, { position:'fixed', zIndex:2147483647, left:'0px', top:'0px',
    width:'20px', height:'20px', borderRadius:'50%', background:'rgba(20,20,20,.85)',
    border:'2px solid #fff', boxShadow:'0 1px 4px rgba(0,0,0,.4)', pointerEvents:'none',
    transform:'translate(-10px,-10px)' });
  document.documentElement.appendChild(c);
  window.__cur = c;
  window.__setCur = (x,y) => { c.style.left = x+'px'; c.style.top = y+'px'; };
})();`;

function cursorState() {
  const st = { x: VW / 2, y: VH / 2 };
  return {
    async ensure(page) { await page.evaluate(CURSOR_JS); await page.evaluate(([x, y]) => window.__setCur(x, y), [st.x, st.y]); },
    async moveTo(page, x, y, steps = 40) {
      const sx = st.x, sy = st.y;
      for (let i = 1; i <= steps; i++) {
        const t = i / steps;
        const e = t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2; // easeInOutQuad
        const nx = sx + (x - sx) * e, ny = sy + (y - sy) * e;
        await page.mouse.move(nx, ny);
        await page.evaluate(([px, py]) => window.__setCur && window.__setCur(px, py), [nx, ny]);
        await page.waitForTimeout(12);
      }
      st.x = x; st.y = y;
    },
    // press at the current position, drag to (x,y) in visible steps, release.
    // Starts with a small nudge so Angular CDK cdkDrag registers the pickup
    // (it ignores instant/teleport drags).
    async dragTo(page, x, y, steps = 30) {
      await page.mouse.move(st.x, st.y);
      await page.mouse.down();
      for (let k = 1; k <= 6; k++) { const ny = st.y - k * 2; await page.mouse.move(st.x, ny); await page.evaluate(([px, py]) => window.__setCur && window.__setCur(px, py), [st.x, ny]); await page.waitForTimeout(22); }
      const sx = st.x, sy = st.y - 12;
      for (let i = 1; i <= steps; i++) { const t = i / steps; const nx = sx + (x - sx) * t, ny = sy + (y - sy) * t; await page.mouse.move(nx, ny); await page.evaluate(([px, py]) => window.__setCur && window.__setCur(px, py), [nx, ny]); await page.waitForTimeout(18); }
      await page.mouse.up();
      st.x = x; st.y = y;
    },
  };
}

// centre of an element's bounding box, for driving the cursor to it
async function centerOf(locator) {
  const b = await locator.boundingBox();
  if (!b) throw new Error('element has no bounding box (not visible?)');
  return [Math.round(b.x + b.width / 2), Math.round(b.y + b.height / 2)];
}

// accept either a raw public id or a full issue URL and return the id
function parseIssueRef(input) {
  const s = String(input).trim();
  const m = s.match(/\/issue\/([^/?#]+)/); // .../project/<p>/issue/<idIssuePublic>
  if (m) return m[1];
  return (s.split('?')[0].split('#')[0].replace(/\/+$/, '').split('/').pop() || s);
}

function promptLine(question) {
  return new Promise((resolve) => {
    const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
    rl.question(question, (a) => { rl.close(); resolve(a.trim()); });
  });
}

// wait for whichever locator becomes visible first; returns its index, or -1 on timeout
async function raceVisible(locators, timeout) {
  return await Promise.race(
    locators.map((loc, i) => loc.first().waitFor({ state: 'visible', timeout }).then(() => i))
  ).catch(() => -1);
}

// open the Split dialog on an issue (gear menu → Split) and type an optional hint
async function openSplitDialog(page, idIssue, hint) {
  await gotoApp(page, `${BASE}/project/${PROJECT}/issue/${idIssue}`); await settle(page);
  await page.locator('app-issue-info button:has(.tabler-icon-settings)').first().click();
  await page.waitForTimeout(400);
  await page.getByText('Split', { exact: true }).first().click(); // menu item
  await page.locator('.ui-dialog').first().waitFor({ state: 'visible', timeout: 8000 });
  await page.waitForTimeout(400);
  if (hint) {
    const box = page.locator('.ui-dialog').first().locator('textarea, input').first();
    if (await box.count()) { await box.fill(hint); await page.waitForTimeout(300); }
  }
}

// open a run issue and element-screenshot the first element matching `sel`
async function shootSel(page, file, sel, idIssue, timeout = 12000) {
  await gotoApp(page, `${BASE}/project/${PROJECT}/issue/${idIssue}`); await settle(page);
  const el = page.locator(sel).first();
  await el.waitFor({ state: 'visible', timeout });
  await el.scrollIntoViewIfNeeded(); await page.waitForTimeout(400);
  await el.screenshot({ path: path.join(IMG, file) });
}

// open a specific issue, expand its Task-quality panel, element-screenshot it
async function shootQuality(page, file, idIssue) {
  await gotoApp(page, `${BASE}/project/${PROJECT}/issue/${idIssue}`);
  await settle(page);
  const panel = page.locator('.quality-panel').first();
  await panel.waitFor({ state: 'visible', timeout: 10000 });
  const collapsed = await panel.evaluate((el) => el.classList.contains('quality-panel--collapsed'));
  if (collapsed) { await page.locator('.quality-panel__header').first().click(); await page.waitForTimeout(800); }
  await panel.scrollIntoViewIfNeeded(); await page.waitForTimeout(400);
  await panel.screenshot({ path: path.join(IMG, file) });
}

// ─────────────────────────────── shot registry ───────────────────────────────
// kind: 'png' shots share one logged-in context; 'webm' shots each get their own
// context with recordVideo. run(page, ctx) where ctx has {cursor, ...helpers}.

const SHOTS = [
  // --- pre-auth ---
  { id: 'login', file: 'login.png', kind: 'png', pre: true, desc: 'Login page (pre-auth)',
    run: async (page) => { await page.goto(`${BASE}/login`, { waitUntil: 'networkidle' }); await settle(page); await page.screenshot({ path: path.join(IMG, 'login.png') }); } },

  // --- the five issue views ---
  ...[['table', 'view-table.png'], ['calendar', 'view-calendar.png']]
    .map(([v, file]) => ({ id: `view-${v}`, file, kind: 'png', desc: `Issue ${v} view`,
      run: async (page) => { await gotoApp(page, `${BASE}/project/${PROJECT}/issue/view/${v}`); await settle(page); await shoot(page, file); } })),

  // gantt is captured with the Day zoom + Compact density (toolbar Hour|Quarter
  // Day|Day|Week|Month and Comfortable|Compact); exact-text avoids "Quarter Day"
  { id: 'view-gantt', file: 'view-gantt.png', kind: 'png', desc: 'Issue gantt view (Day zoom, Compact density)',
    run: async (page) => {
      await gotoApp(page, `${BASE}/project/${PROJECT}/issue/view/gantt`); await settle(page);
      for (const label of ['Day', 'Compact']) {
        const btn = page.getByText(label, { exact: true }).first();
        if (await btn.count()) { await btn.click(); await page.waitForTimeout(700); }
      }
      await settle(page); await shoot(page, 'view-gantt.png');
    } },

  // kanban is captured on the Swimlane layout (toolbar Columns | Swimlane)
  { id: 'view-kanban', file: 'view-kanban.png', kind: 'png', desc: 'Issue kanban view (Swimlane layout)',
    run: async (page) => {
      await gotoApp(page, `${BASE}/project/${PROJECT}/issue/view/kanban`); await settle(page);
      await selectSwimlane(page);
      await shoot(page, 'view-kanban.png');
    } },

  { id: 'hero', file: 'hero.png', kind: 'png', desc: 'Hero (gantt on Day zoom, Compact density)',
    run: async (page) => {
      await gotoApp(page, `${BASE}/project/${PROJECT}/issue/view/gantt`); await settle(page);
      for (const label of ['Day', 'Compact']) {
        const btn = page.getByText(label, { exact: true }).first();
        if (await btn.count()) { await btn.click(); await page.waitForTimeout(700); }
      }
      await settle(page); await shoot(page, 'hero.png');
    } },

  { id: 'project-overview', file: 'project-overview.png', kind: 'png', desc: 'Project overview dashboard',
    run: async (page) => { await gotoApp(page, `${BASE}/project/${PROJECT}/view`); await settle(page); await shoot(page, 'project-overview.png'); } },

  { id: 'issue-detail', file: 'issue-detail.png', kind: 'png', desc: 'Issue detail (incl. participants card)',
    run: async (page) => {
      if (ISSUE) { await gotoApp(page, `${BASE}/project/${PROJECT}/issue/${ISSUE}`); }
      else {
        await gotoApp(page, `${BASE}/project/${PROJECT}/issue/view/table`); await settle(page);
        const row = page.locator('tbody tr a, .p-datatable-tbody tr').first();
        await row.waitFor({ state: 'visible', timeout: 15000 }); await row.click();
        await page.waitForLoadState('networkidle');
      }
      await settle(page); await shoot(page, 'issue-detail.png');
    } },

  // --- task-quality panels: same UI, different issue. Which issue is the shot
  // pointed at decides high vs low — so these need an idIssuePublic (from the
  // issue URL). Provide it via env, `quality-high=<id|url>` on the CLI, or the
  // interactive prompt. Element-screenshots the expanded quality panel. ---
  { id: 'quality-high', file: 'quality-high.png', kind: 'png', needsIssue: true, envIssue: 'CAPTURE_QUALITY_HIGH_ISSUE',
    desc: 'Task-quality panel, high score (needs an issue id / URL)',
    run: async (page, _ctx, shot) => shootQuality(page, 'quality-high.png', shot._issueId) },
  { id: 'quality-low', file: 'quality-low.png', kind: 'png', needsIssue: true, envIssue: 'CAPTURE_QUALITY_LOW_ISSUE',
    desc: 'Task-quality panel, low score + suggestions (needs an issue id / URL)',
    run: async (page, _ctx, shot) => shootQuality(page, 'quality-low.png', shot._issueId) },

  // Participants card with ≥2 people: assign the task to a human (NOT a bot) and
  // post a comment (author becomes a participant), then expand the panel.
  // MUTATES issue #DEMO_ISSUE (assignee + a comment).
  { id: 'participants-panel', file: 'participants-panel.png', kind: 'png', desc: 'Participants card expanded, 2 participants — MUTATES (assign + comment)',
    run: async (page) => {
      await gotoApp(page, `${BASE}/project/${PROJECT}/issue/${DEMO_ISSUE}`); await settle(page);
      // assign to the 2nd human user via the assignee dropdown (avoid bots)
      await page.locator('app-user-dropdown').first().click(); await page.waitForTimeout(500);
      await page.getByText(USER2_NAME, { exact: true }).first().click(); await page.waitForTimeout(900);
      // post a comment in the activity composer (author → participant)
      const editor = page.locator('.feed-editor .editor-input').first();
      await editor.scrollIntoViewIfNeeded(); await editor.click();
      await editor.type('Taking a look — pairing with ' + USER2_NAME + ' on this.');
      await page.keyboard.press('Shift+Enter'); await page.waitForTimeout(1300);
      // expand + screenshot the participants panel (screenshot the inner
      // .participants-panel; the host element has no layout box)
      const root = page.locator('app-issue-participants .participants-panel').first();
      await root.waitFor({ state: 'visible', timeout: 8000 });
      if (await page.locator('app-issue-participants .participants-panel--collapsed').count()) {
        await page.locator('app-issue-participants .participants-panel__header').first().click(); await page.waitForTimeout(600);
      }
      await root.scrollIntoViewIfNeeded(); await page.waitForTimeout(400);
      await root.screenshot({ path: path.join(IMG, 'participants-panel.png') });
    } },

  // Mention autocomplete: type "@" + a letter in the comment composer to open the
  // picker. Read-only (nothing posted).
  { id: 'mention-picker', file: 'mention-picker.png', kind: 'png', desc: 'Mention picker autocomplete in the comment composer',
    run: async (page) => {
      await gotoApp(page, `${BASE}/project/${PROJECT}/issue/${DEMO_ISSUE}`); await settle(page);
      const composer = page.locator('.feed-editor').first();
      const editor = composer.locator('.editor-input').first();
      await composer.scrollIntoViewIfNeeded(); await editor.click();
      await editor.type('@r'); // query → shows the Roberts etc.
      const picker = page.locator('.mention-picker').first();
      await picker.waitFor({ state: 'visible', timeout: 6000 });
      await page.waitForTimeout(500);
      // the picker renders above the composer — clip a region covering both
      const pb = await picker.boundingBox(), cb = await composer.boundingBox();
      const x = Math.min(pb.x, cb.x) - 8, y = Math.min(pb.y, cb.y) - 8;
      const w = Math.max(pb.x + pb.width, cb.x + cb.width) + 8 - x;
      const h = Math.max(pb.y + pb.height, cb.y + cb.height) + 8 - y;
      await page.screenshot({ path: path.join(IMG, 'mention-picker.png'), clip: { x, y, width: w, height: h } });
    } },

  // Posted comment containing a mention → rendered as a chip. MUTATES (posts a comment).
  { id: 'mention-chip', file: 'mention-chip.png', kind: 'png', desc: 'Rendered mention chip in a posted comment — MUTATES (posts a comment)',
    run: async (page) => {
      await gotoApp(page, `${BASE}/project/${PROJECT}/issue/${DEMO_ISSUE}`); await settle(page);
      const editor = page.locator('.feed-editor .editor-input').first();
      await editor.scrollIntoViewIfNeeded(); await editor.click();
      await editor.type('@' + USER2_NAME.split(' ')[0]); // "@Robert"
      await page.locator('.mention-picker__item').first().waitFor({ state: 'visible', timeout: 6000 });
      await page.locator('.mention-picker__item').first().click(); // insert the mention token
      await editor.type(' can you review this one?');
      await page.keyboard.press('Shift+Enter'); await page.waitForTimeout(1400);
      // screenshot the posted comment that holds the chip
      const chip = page.locator('.mention-chip').first();
      await chip.waitFor({ state: 'visible', timeout: 6000 });
      const item = page.locator('app-activity-comment-item').filter({ has: page.locator('.mention-chip') }).first();
      await item.scrollIntoViewIfNeeded(); await page.waitForTimeout(300);
      await item.screenshot({ path: path.join(IMG, 'mention-chip.png') });
    } },

  // Time tracked by TWO users on the same task, shown in the activity feed filtered
  // to the clock (Time) chip. Logs in as each user to log their time.
  // MUTATES issue #DEMO_ISSUE (two time entries).
  { id: 'tracker', file: 'tracker.png', kind: 'png', desc: 'Feed filtered to Time chip: two users\' tracked time — MUTATES (logs time)',
    run: async (page) => {
      const logTime = async (value) => {
        await gotoApp(page, `${BASE}/project/${PROJECT}/issue/${DEMO_ISSUE}`); await settle(page);
        const input = page.locator('app-tracker input[uiInput], app-tracker input[type="text"]').first();
        await input.scrollIntoViewIfNeeded(); await input.click();
        await input.fill(value);
        await input.press('Enter').catch(() => {});
        // click the check (save) button next to the tracker input
        await page.locator('app-tracker').first().locator('button').first().click().catch(() => {});
        await page.waitForTimeout(1200);
      };
      // user 1 (already logged in as default) logs time
      await logTime('45m');
      // user 2 logs time on the same task
      await loginAs(page, EMAIL2, PASSWORD2);
      await logTime('1h 15m');
      // view the feed filtered to Time and screenshot it
      await gotoApp(page, `${BASE}/project/${PROJECT}/issue/${DEMO_ISSUE}`); await settle(page);
      const feed = page.locator('app-issue-activity-feed').first();
      await feed.locator('.feed-header .chip').filter({ hasText: 'Time' }).first().click();
      await page.waitForTimeout(800);
      await feed.scrollIntoViewIfNeeded(); await page.waitForTimeout(400);
      await feed.screenshot({ path: path.join(IMG, 'tracker.png') });
      // restore the default session so later shots in the batch run as user 1
      await loginAs(page, EMAIL, PASSWORD);
    } },

  // Split dialog step 1 (before generate): the hint input. Cancelled — nothing generated.
  { id: 'split-dialog-before-generate', file: 'split-dialog-before-generate.png', kind: 'png', desc: 'Split dialog before Generate (hint step)',
    run: async (page) => {
      await openSplitDialog(page, SPLIT_ISSUE, 'separate parsing, formatting and error handling');
      await page.locator('.ui-dialog').first().screenshot({ path: path.join(IMG, 'split-dialog-before-generate.png') });
      await page.getByRole('button', { name: 'Cancel' }).first().click().catch(() => page.keyboard.press('Escape'));
    } },

  // Split dialog step 2 (after generate): calls AI, waits for the generated child
  // tasks (review step). Does NOT accept/save. Handles AI timeout/failure.
  { id: 'split-dialog-after-generate', file: 'split-dialog-after-generate.png', kind: 'png', desc: 'Split dialog after Generate (AI children) — calls AI, not saved',
    run: async (page) => {
      await openSplitDialog(page, SPLIT_ISSUE, 'separate parsing, formatting and error handling');
      await page.getByRole('button', { name: 'Split with AI' }).first().click();
      // wait for either the review step (children) or an error, up to 2 min (AI can be slow)
      const review = page.locator('.children-list');
      const error = page.getByText(/Could not generate|wait a moment/i);
      const start = await raceVisible([review, error], 120000);
      if (start !== 0) throw new Error('AI split failed or timed out (no child tasks) — try again');
      await page.waitForTimeout(1000); // let all child rows render
      await page.locator('.ui-dialog').first().screenshot({ path: path.join(IMG, 'split-dialog-after-generate.png') });
      // close WITHOUT accepting (do not click "Accept (N)")
      await page.getByRole('button', { name: 'Cancel' }).first().click().catch(() => page.keyboard.press('Escape'));
    } },

  // === AGENT RUN shots — capture from an issue whose agent run you've driven to
  // "wait for merge" (pr_open). All share one issue id (CAPTURE_RUN_ISSUE / prompt
  // once). Read-only: they only screenshot the resulting comments/card. The whole
  // agent-workflow.webm clip still needs live recording (not scriptable here). ===
  { id: 'stage-brainstorming', file: 'stage-brainstorming.png', kind: 'png', needsIssue: true, envIssue: 'CAPTURE_RUN_ISSUE', promptLabel: 'run issue',
    desc: 'Brainstorming stage comment (needs a run issue id / URL)',
    run: async (page, _c, shot) => {
      await gotoApp(page, `${BASE}/project/${PROJECT}/issue/${shot._issueId}`); await settle(page);
      let el = page.locator('.comment-card.kind-brainstorming-complete').first();
      if (!(await el.count())) el = page.locator('.comment-card.kind-brainstorming-question').first();
      await el.waitFor({ state: 'visible', timeout: 12000 });
      await el.scrollIntoViewIfNeeded(); await page.waitForTimeout(400);
      await el.screenshot({ path: path.join(IMG, 'stage-brainstorming.png') });
    } },
  { id: 'stage-design', file: 'stage-design.png', kind: 'png', needsIssue: true, envIssue: 'CAPTURE_RUN_ISSUE', promptLabel: 'run issue',
    desc: 'Design stage comment', run: async (page, _c, shot) => shootSel(page, 'stage-design.png', '.comment-card.kind-design', shot._issueId) },
  { id: 'stage-plan', file: 'stage-plan.png', kind: 'png', needsIssue: true, envIssue: 'CAPTURE_RUN_ISSUE', promptLabel: 'run issue',
    desc: 'Implementation plan stage comment (diffs)', run: async (page, _c, shot) => shootSel(page, 'stage-plan.png', '.comment-card.kind-implementation-plan', shot._issueId) },
  { id: 'run-pr', file: 'run-pr.png', kind: 'png', needsIssue: true, envIssue: 'CAPTURE_RUN_ISSUE', promptLabel: 'run issue',
    desc: 'PR-pushed comment with the PR link (pr_open)', run: async (page, _c, shot) => shootSel(page, 'run-pr.png', '.comment-card.kind-pull-request-pushed', shot._issueId) },
  { id: 'message-mockup', file: 'message-mockup.png', kind: 'png', needsIssue: true, envIssue: 'CAPTURE_RUN_ISSUE', promptLabel: 'run issue',
    desc: 'Design comment mockup cards (one selected) + surroundings', run: async (page, _c, shot) => shootSel(page, 'message-mockup.png', '.comment-card.kind-design', shot._issueId) },
  { id: 'stage-mockup-preview', file: 'stage-mockup-preview.png', kind: 'png', needsIssue: true, envIssue: 'CAPTURE_RUN_ISSUE', promptLabel: 'run issue',
    desc: 'Opened sandboxed mockup preview modal',
    run: async (page, _c, shot) => {
      await gotoApp(page, `${BASE}/project/${PROJECT}/issue/${shot._issueId}`); await settle(page);
      const card = page.locator('.mockup-card').first();
      await card.waitFor({ state: 'visible', timeout: 12000 });
      await card.scrollIntoViewIfNeeded(); await card.click();
      const modal = page.locator('.ui-dialog').filter({ has: page.locator('.mockup-frame-wrap') }).first();
      await modal.waitFor({ state: 'visible', timeout: 8000 });
      await page.waitForTimeout(1200); // let the sandboxed iframe render
      await modal.screenshot({ path: path.join(IMG, 'stage-mockup-preview.png') });
      await page.keyboard.press('Escape');
    } },
  { id: 'run-card', file: 'run-card.png', kind: 'png', needsIssue: true, envIssue: 'CAPTURE_RUN_ISSUE', promptLabel: 'run issue',
    desc: 'Agent run card with stage timeline + phase badge',
    run: async (page, _c, shot) => {
      await gotoApp(page, `${BASE}/project/${PROJECT}/issue/${shot._issueId}`); await settle(page);
      const card = page.locator('.agent-run-card').first();
      await card.waitFor({ state: 'visible', timeout: 12000 });
      if (!(await card.evaluate((el) => el.classList.contains('is-open')).catch(() => true))) {
        await card.locator('.card-summary').first().click().catch(() => {}); await page.waitForTimeout(600);
      }
      await card.scrollIntoViewIfNeeded(); await page.waitForTimeout(400);
      await card.screenshot({ path: path.join(IMG, 'run-card.png') });
    } },

  { id: 'admin-users', file: 'admin-users.png', kind: 'png', desc: 'Admin → users list',
    run: async (page) => { await gotoApp(page, `${BASE}/admin/users`); await settle(page); await shoot(page, 'admin-users.png'); } },

  // Teams live on the /admin/users page as a second panel (<app-admin-teams>),
  // there is no /admin/teams route. Element-screenshot just the teams panel.
  { id: 'admin-teams', file: 'admin-teams.png', kind: 'png', desc: 'Admin → teams panel, first team selected (shows its members)',
    run: async (page) => {
      await gotoApp(page, `${BASE}/admin/users`); await settle(page);
      const panel = page.locator('app-admin-teams').first();
      await panel.waitFor({ state: 'visible', timeout: 10000 });
      // select the first team so the right pane shows its assigned users
      const firstTeam = panel.locator('.team-list > div').first();
      if (await firstTeam.count()) { await firstTeam.click(); await page.waitForTimeout(700); }
      await panel.scrollIntoViewIfNeeded(); await page.waitForTimeout(400);
      await panel.screenshot({ path: path.join(IMG, 'admin-teams.png') });
    } },

  // New-user dialog filled for a human (GoT name, matching the demo users), then
  // Cancelled — no user is created
  { id: 'create-user-dialog', file: 'create-user-dialog.png', kind: 'png', desc: 'New user dialog, filled for a human (not saved)',
    run: async (page) => {
      await gotoApp(page, `${BASE}/admin/users`); await settle(page);
      await page.getByRole('button', { name: 'Create user' }).first().click();
      const dialog = page.locator('.ui-dialog').first();
      await dialog.waitFor({ state: 'visible', timeout: 8000 });
      await page.fill('#cu-name', 'Arya Stark');
      await page.fill('#cu-email', 'arya.stark@test.sk');
      await page.fill('#cu-password', 'kreslo');
      await page.waitForTimeout(400);
      await dialog.screenshot({ path: path.join(IMG, 'create-user-dialog.png') });
      await page.getByRole('button', { name: 'Cancel' }).first().click().catch(() => page.keyboard.press('Escape'));
    } },

  // Same dialog with the Bot toggle on → shows the Gateway URL field; pointed at
  // the qwen goose gateway. Cancelled — no bot is created.
  { id: 'bot-create', file: 'bot-create.png', kind: 'png', desc: 'New user dialog, Bot toggle on, qwen gateway (not saved)',
    run: async (page) => {
      await gotoApp(page, `${BASE}/admin/users`); await settle(page);
      await page.getByRole('button', { name: 'Create user' }).first().click();
      const dialog = page.locator('.ui-dialog').first();
      await dialog.waitFor({ state: 'visible', timeout: 8000 });
      await page.locator('#cu-isbot').check(); await page.waitForTimeout(400); // reveal bot fields
      await page.fill('#cu-name', 'Qwen');
      await page.fill('#cu-gateway-url', 'http://gateway-goose-qwen-cloud:9090');
      await page.waitForTimeout(400);
      await dialog.screenshot({ path: path.join(IMG, 'bot-create.png') });
      await page.getByRole('button', { name: 'Cancel' }).first().click().catch(() => page.keyboard.press('Escape'));
    } },

  // Actually creates a temp bot (its keys dialog auto-opens with the minted key),
  // screenshots that dialog, then DELETES the temp bot again (cleanup).
  { id: 'bot-keys', file: 'bot-keys.png', kind: 'png', desc: 'New bot API-keys dialog — creates a TEMP bot, shoots, then deletes it',
    run: async (page) => {
      const BOT_NAME = 'TEMP Screenshot Bot';
      await gotoApp(page, `${BASE}/admin/users`); await settle(page);
      await page.getByRole('button', { name: 'Create user' }).first().click();
      const create = page.locator('.ui-dialog').first();
      await create.waitFor({ state: 'visible', timeout: 8000 });
      await page.locator('#cu-isbot').check(); await page.waitForTimeout(400);
      await page.fill('#cu-name', BOT_NAME);
      await page.fill('#cu-gateway-url', 'http://gateway-goose-qwen-cloud:9090');
      await page.getByRole('button', { name: 'Create', exact: true }).first().click();
      // the bot lands straight in its keys window with the minted token revealed
      await page.getByRole('button', { name: 'Close', exact: true }).first().waitFor({ state: 'visible', timeout: 10000 });
      await page.waitForTimeout(1000);
      await page.locator('.ui-dialog').first().screenshot({ path: path.join(IMG, 'bot-keys.png') });
      await page.getByRole('button', { name: 'Close', exact: true }).first().click().catch(() => page.keyboard.press('Escape'));
      await page.waitForTimeout(800);
      // cleanup: delete the temp bot (match its exact row, trash → confirm Yes)
      try {
        await gotoApp(page, `${BASE}/admin/users`); await settle(page);
        const row = page.locator('tr', { hasText: BOT_NAME }).first();
        await row.waitFor({ state: 'visible', timeout: 8000 });
        await row.locator('button:has(.tabler-icon-trash)').first().click();
        await page.getByRole('button', { name: 'Yes', exact: true }).first().click();
        await page.waitForTimeout(1000);
        console.log('    (temp bot deleted)');
      } catch (e) { console.warn(`    ! cleanup failed — delete "${BOT_NAME}" manually (${String(e.message).split('\n')[0]})`); }
    } },

  // element-screenshot the whole members component so both the USERS subblock
  // and the TEAMS subblock (further down) are visible without manual scrolling
  { id: 'project-members', file: 'project-members.png', kind: 'png', desc: 'Project settings → members (users + teams)',
    run: async (page) => {
      await gotoApp(page, `${BASE}/project/${PROJECT}/settings`); await settle(page);
      const comp = page.locator('app-project-members').first();
      await comp.waitFor({ state: 'visible', timeout: 15000 });
      await comp.scrollIntoViewIfNeeded(); await page.waitForTimeout(500);
      await comp.screenshot({ path: path.join(IMG, 'project-members.png') });
    } },

  { id: 'git-integration-settings', file: 'git-integration-settings.png', kind: 'png', desc: 'Project settings → git integrations (list)',
    run: async (page) => { await gotoApp(page, `${BASE}/project/${PROJECT}/settings`); const t = page.getByText(/git/i).first(); await t.waitFor({ state: 'visible', timeout: 15000 }); await t.click(); await settle(page); await shoot(page, 'git-integration-settings.png'); } },

  // open the "New integration" dialog, fill it, element-screenshot it, then
  // Cancel WITHOUT saving (nothing is created)
  { id: 'git-integration-form', file: 'git-integration-form.png', kind: 'png', desc: 'Git integration create form, filled then cancelled (not saved)',
    run: async (page) => {
      await gotoApp(page, `${BASE}/project/${PROJECT}/settings`); await settle(page);
      const newBtn = page.getByRole('button', { name: 'New integration' }).first();
      await newBtn.waitFor({ state: 'visible', timeout: 15000 });
      await newBtn.scrollIntoViewIfNeeded(); await newBtn.click();
      const dialog = page.locator('.ui-dialog').first();
      await dialog.waitFor({ state: 'visible', timeout: 8000 });
      await page.fill('#gi-name', 'DevToolbox · GitHub');
      await page.fill('#gi-base-url', 'https://github.com');
      await page.fill('#gi-repo-path', 'acme/devtoolbox');
      await page.fill('#gi-token', 'ghp_exampleTokenNotReal0000000000000000');
      await page.waitForTimeout(500);
      await dialog.screenshot({ path: path.join(IMG, 'git-integration-form.png') });
      await page.getByRole('button', { name: 'Cancel' }).first().click().catch(() => page.keyboard.press('Escape'));
    } },

  // --- command palette: highlight a task in the table (primes the palette's
  // issue context), then open in commands mode (>) so the contextual State /
  // Severity / Assign actions for that task are shown ---
  { id: 'command-palette', file: 'command-palette.png', kind: 'png', desc: 'Command palette in > (commands) mode over a highlighted task — State/Severity/Assign',
    run: async (page) => {
      await gotoApp(page, `${BASE}/project/${PROJECT}/issue/view/table`); await settle(page);
      await page.getByText('Tasks', { exact: true }).first().click().catch(() => {}); // focus off inputs
      await page.keyboard.press('ArrowDown'); await page.waitForTimeout(300); // highlight 1st row
      await page.keyboard.press('ArrowDown'); await page.waitForTimeout(400); // → 2nd row (sets ctx.issue)
      await page.keyboard.press('Meta+KeyK'); await page.waitForTimeout(600);
      // ">set" surfaces the "Set state → …" and "Set severity → …" groups
      await page.keyboard.type('>set'); await page.waitForTimeout(900);
      await shoot(page, 'command-palette.png');
      await page.keyboard.press('Escape');
    } },

  // --- command palette flow clip: calendar → (palette /) navigate to Tasks →
  // highlight the 2nd task → change its severity → change its assignee, all via
  // the palette. MUTATES the 2nd task's severity + assignee (reversible). ---
  { id: 'command-palette-clip', file: 'command-palette.webm', kind: 'webm', desc: 'Palette flow clip: nav + change severity + assignee of the 2nd task — MUTATES it',
    run: async (page, { cursor, mark }) => {
      await gotoApp(page, `${BASE}/project/${PROJECT}/issue/view/calendar`); await settle(page);
      await cursor.ensure(page); mark(); // clip starts on the calendar (login trimmed off)
      await page.waitForTimeout(700);
      // 1) navigate calendar → Tasks (table) via the palette's `/` navigation mode
      await page.keyboard.press('/'); await page.waitForTimeout(600); await cursor.ensure(page);
      for (const ch of 'Tasks') { await page.keyboard.type(ch); await page.waitForTimeout(130); }
      await page.waitForTimeout(700); await page.keyboard.press('Enter');
      await settle(page); await cursor.ensure(page);
      // 2) highlight the 2nd task row
      await page.getByText('Tasks', { exact: true }).first().click().catch(() => {});
      await page.keyboard.press('ArrowDown'); await page.waitForTimeout(400);
      await page.keyboard.press('ArrowDown'); await page.waitForTimeout(700);
      // 3) change severity via the palette (> commands mode)
      await page.keyboard.press('Meta+KeyK'); await page.waitForTimeout(500); await cursor.ensure(page);
      await page.keyboard.type('>'); await page.waitForTimeout(300);
      for (const ch of 'severity') { await page.keyboard.type(ch); await page.waitForTimeout(120); }
      await page.waitForTimeout(800); await page.keyboard.press('Enter'); await page.waitForTimeout(1000);
      // 4) change assignee via the palette
      await page.keyboard.press('Meta+KeyK'); await page.waitForTimeout(500); await cursor.ensure(page);
      await page.keyboard.type('>'); await page.waitForTimeout(300);
      for (const ch of 'assign') { await page.keyboard.type(ch); await page.waitForTimeout(120); }
      await page.waitForTimeout(800); await page.keyboard.press('Enter'); await page.waitForTimeout(1200);
    } },

  // --- sprint board (kanban with the always-on sprint tab strip: current
  // cycle lane + Backlog lane). NOTE: TASKS.md's "layout switch to Sprints +
  // scope chip" is stale — the lanes are always present; there is no toggle. ---
  { id: 'sprint-board', file: 'sprint-board.png', kind: 'png', desc: 'Board with sprint tab strip (cycle + Backlog lanes)',
    run: async (page) => {
      await gotoApp(page, `${BASE}/project/${PROJECT}/issue/view/kanban`); await settle(page);
      await selectSwimlane(page);
      await page.locator('.sprint-tabs').first().waitFor({ state: 'visible', timeout: 10000 }).catch(() => {});
      await shoot(page, 'sprint-board.png');
    } },

  // --- sprint create: click the "＋ Sprint" button in the tab strip, which
  // opens the sprint dialog directly (no intermediate menu — TASKS.md's "chip
  // menu with ＋ New sprint row" is stale). Screenshots the OPEN dialog and
  // closes it WITHOUT submitting, so no sprint is created. ---
  { id: 'sprint-create', file: 'sprint-create.png', kind: 'png', desc: 'Sprint create dialog (opened from ＋ Sprint, not submitted)',
    run: async (page) => {
      await gotoApp(page, `${BASE}/project/${PROJECT}/issue/view/kanban`); await settle(page);
      await selectSwimlane(page);
      // The add-sprint control is a <ui-button>; the backlog tab shares its
      // .sprint-tabs__pinned class, so scope by element to avoid matching that.
      const add = page.locator('ui-button.sprint-tabs__pinned').first();
      await add.waitFor({ state: 'visible', timeout: 10000 });
      await add.scrollIntoViewIfNeeded();
      await add.click();
      const dialog = page.locator('.ui-dialog').first();
      await dialog.waitFor({ state: 'visible', timeout: 8000 });
      await page.waitForTimeout(700);
      await shoot(page, 'sprint-create.png');
      await page.keyboard.press('Escape'); // close without creating
    } },

  // --- gantt WBS row reorder clip: drag a Scheduled row up, then reload to
  // show the new order persisted (gantt_rank). MUTATES row order (reversible). ---
  { id: 'gantt-reorder', file: 'gantt-reorder.webm', kind: 'webm', desc: 'Drag a WBS Scheduled row to reorder (Day+Compact, persisted) — MUTATES order',
    run: async (page, { cursor, mark }) => {
      await gotoApp(page, `${BASE}/project/${PROJECT}/issue/view/gantt`); await settle(page);
      await selectGanttDayCompact(page); // set the view up BEFORE the recorded part
      mark(); // clip starts here — login + toggle switching gets trimmed off
      await cursor.ensure(page); await page.waitForTimeout(400);
      const sched = page.getByText('Scheduled', { exact: true }).first();
      if (await sched.count()) { const [sx, sy] = await centerOf(sched); await cursor.moveTo(page, sx, sy, 25); await sched.click(); await page.waitForTimeout(700); }
      const rows = page.locator('.wbs-row');
      await rows.first().waitFor({ state: 'visible', timeout: 10000 });
      const n = await rows.count();
      if (n < 3) throw new Error(`need ≥3 scheduled rows to show a reorder (found ${n})`);
      const src = rows.nth(2), dst = rows.nth(0);
      const [srcX, srcY] = await centerOf(src);
      const dstBox = await dst.boundingBox();
      const targetY = Math.round(dstBox.y + 6);
      await cursor.moveTo(page, srcX, srcY, 30); await page.waitForTimeout(450);
      await cursor.dragTo(page, srcX, targetY, 30); await page.waitForTimeout(900);
      await page.reload({ waitUntil: 'networkidle' }); await settle(page);
      await cursor.ensure(page);
      await page.getByText('Scheduled', { exact: true }).first().click().catch(() => {});
      await page.waitForTimeout(1200);
    } },

  // AI Project Builder (Kickstart) clip: describe → defaults State=New/Severity=Low →
  // Generate Backlog (AI) → bump the parent task to Severity=Medium → Accept All →
  // land on the task table. MUTATES: generates a full backlog on the given project,
  // so it needs an EMPTY project (env CAPTURE_KICKSTART_PROJECT / CLI / prompt).
  { id: 'kickstart', file: 'kickstart.webm', kind: 'webm', needsProject: true, envProject: 'CAPTURE_KICKSTART_PROJECT', promptLabel: 'empty project',
    desc: 'AI Project Builder → generate → parent→Medium → accept → task table — MUTATES (fills the project)',
    run: async (page, { cursor, mark }, shot) => {
      const P = shot._projectId;
      const DESC = 'DevToolbox — a single-page web app that hosts a collection of small, independent developer utilities. '
        + 'Each tool is fully self-contained in its own folder under tools/<tool-name>/ with its own UI, logic, and tests, '
        + 'and shares no files with other tools. Build these tools as separate, independent tasks: JSON Formatter & Validator, '
        + 'Base64 Encode/Decode, JWT Decoder, Hash Generator (MD5/SHA-256), Color Converter (HEX/RGB/HSL). '
        + 'Add one parent task "App shell & tool registry" that the individual tools plug into without editing shared code.';
      await gotoApp(page, `${BASE}/project/${P}/project-builder`); await settle(page);
      await cursor.ensure(page); mark(); // clip starts on the builder (login trimmed)
      // 1) describe the project
      const ta = page.locator('textarea').first();
      await ta.click(); await ta.fill(DESC); await page.waitForTimeout(600);
      // 2) defaults: State = New, Severity = Low (Generate stays disabled until both are set)
      await pickDropdownOption(page, page.locator('app-state-dropdown').first(), 'New');
      await pickDropdownOption(page, page.locator('app-severity-dropdown').first(), 'Low');
      await page.waitForTimeout(400);
      // 3) generate (AI) — wait for the staging tree (up to ~2.5 min)
      await page.getByRole('button', { name: 'Generate Backlog' }).first().click();
      await page.locator('app-staged-issue-tree').first().waitFor({ state: 'visible', timeout: 240000 })
        .catch(() => { throw new Error('kickstart AI did not return a backlog (it errored or timed out server-side — the kickstart AI is flaky; just re-run)'); });
      await page.waitForTimeout(1500);
      // 4) bump the parent (first) task to Severity = Medium
      await pickDropdownOption(page, page.locator('app-staged-issue-tree app-severity-dropdown').first(), 'Medium');
      await page.waitForTimeout(800);
      // 5) accept the backlog
      await page.getByRole('button', { name: /Accept All|Accept/ }).first().click();
      await page.getByText(/Backlog Created/i).first().waitFor({ state: 'visible', timeout: 60000 }).catch(() => {});
      await page.waitForTimeout(1000);
      // 6) go to the task table (natural CTA, fallback to direct nav) — end of clip
      await page.locator('app-project-builder-step-success button').first().click().catch(() => {});
      await page.waitForTimeout(1500);
      if (!/\/issue\/view/.test(page.url())) { await gotoApp(page, `${BASE}/project/${P}/issue/view/table`); }
      await settle(page); await page.waitForTimeout(1500);
    } },
];

// ───────────────────────────────── runners ─────────────────────────────────

async function newContext({ video = false } = {}) {
  const browser = await chromium.launch();
  const opts = { viewport: { width: VW, height: VH }, deviceScaleFactor: video ? 1 : 2 };
  if (video) opts.recordVideo = { dir: IMG, size: { width: VW, height: VH } };
  const context = await browser.newContext(opts);
  const page = await context.newPage();
  return { browser, context, page };
}

async function runPngShots(shots) {
  const { browser, context, page } = await newContext();
  const cursor = cursorState();
  const ctx = { cursor };
  try {
    // pre-auth shots first (no login needed), then log in for the rest
    const pre = shots.filter((s) => s.pre);
    const post = shots.filter((s) => !s.pre);
    for (const s of pre) { await tryRun(s, page, ctx); }
    if (post.length) { await login(page); for (const s of post) { await tryRun(s, page, ctx); } }
  } finally { await context.close(); await browser.close(); }
}

async function runWebmShot(shot) {
  const { browser, context, page } = await newContext({ video: true });
  const cursor = cursorState();
  const t0 = Date.now();
  let markMs = null;
  // the shot calls ctx.mark() once its interesting part begins (after login and
  // any setup like layout toggles) so that lead-in gets trimmed off the clip
  const mark = () => { if (markMs === null) markMs = Date.now(); };
  try {
    await login(page);
    await tryRun(shot, page, { cursor, mark });
  } finally { await context.close(); await browser.close(); }
  // rename the freshly written .webm to the shot's filename
  const webms = fs.readdirSync(IMG).filter((f) => f.endsWith('.webm')).map((f) => path.join(IMG, f));
  if (!webms.length) return;
  const newest = webms.sort((a, b) => fs.statSync(b).mtimeMs - fs.statSync(a).mtimeMs)[0];
  const dst = path.join(IMG, shot.file);
  if (newest !== dst) { fs.renameSync(newest, dst); }
  // trim the login/setup lead-in (start ~0.4s before the marked moment)
  if (markMs !== null) {
    const offset = Math.max(0, (markMs - t0) / 1000 - 0.4);
    try { if (trimWebm(dst, offset)) console.log(`  ✂  trimmed lead-in (${offset.toFixed(1)}s)`); }
    catch (e) { console.warn(`  ! trim failed (${String(e.message).split('\n')[0]}) — keeping full clip`); }
  }
  console.log(`  🎬 ${shot.file}`);
}

async function tryRun(shot, page, ctx) {
  try { await shot.run(page, ctx, shot); if (shot.kind === 'png') console.log(`  ✓ ${shot.file}`); }
  catch (err) { console.warn(`  ✗ ${shot.file}: ${String(err.message).split('\n')[0]}`); }
}

// resolve an idIssuePublic for shots that need one: env var → CLI `id=value` →
// cached prior answer for the same env key → interactive prompt. Returns false
// (and warns) if none is available. Caching means a batch of agent shots that
// share one envIssue (CAPTURE_RUN_ISSUE) only prompts once.
const issueRefCache = {};
async function resolveIssue(shot, issueMap) {
  let ref = process.env[shot.envIssue] || issueMap[shot.id] || issueRefCache[shot.envIssue];
  if (!ref && process.stdin.isTTY) ref = await promptLine(`  ${shot.promptLabel || shot.id}: paste the issue URL or public id → `);
  if (!ref) { console.warn(`  ✗ ${shot.file}: no issue id (set ${shot.envIssue}, pass ${shot.id}=<id|url>, or run interactively)`); return false; }
  issueRefCache[shot.envIssue] = ref;
  shot._issueId = parseIssueRef(ref);
  return true;
}

// resolve a target project id: env → CLI `id=value` → prompt. Returns false if none.
const projectRefCache = {};
async function resolveProject(shot, issueMap) {
  let ref = process.env[shot.envProject] || issueMap[shot.id] || projectRefCache[shot.envProject];
  if (!ref && process.stdin.isTTY) ref = await promptLine(`  ${shot.promptLabel || shot.id}: enter the project id (empty project — a full backlog will be generated) → `);
  if (!ref) { console.warn(`  ✗ ${shot.file}: no project id (set ${shot.envProject}, pass ${shot.id}=<id>, or run interactively)`); return false; }
  projectRefCache[shot.envProject] = ref;
  shot._projectId = parseIssueRef(ref);
  return true;
}

// open a ui-select-style dropdown and click the option with the given visible
// text. Options render in a CDK overlay (state = <app-state-badge>, severity =
// .dropdown-item), so match by text inside the overlay rather than a fixed class.
async function pickDropdownOption(page, dropdownLocator, optionText) {
  await dropdownLocator.scrollIntoViewIfNeeded();
  await dropdownLocator.click(); await page.waitForTimeout(400);
  await page.locator('.cdk-overlay-container').getByText(optionText, { exact: false }).first().click();
  await page.waitForTimeout(400);
}

async function run(ids, issueMap = {}) {
  let chosen = SHOTS.filter((s) => ids.includes(s.id));
  if (!chosen.length) { console.error('nothing selected'); return; }
  // resolve issue ids up front (may prompt) and drop shots that can't be resolved
  const resolved = [];
  for (const s of chosen) {
    if (s.needsIssue && !(await resolveIssue(s, issueMap))) continue;
    if (s.needsProject && !(await resolveProject(s, issueMap))) continue;
    resolved.push(s);
  }
  chosen = resolved;
  if (!chosen.length) { console.log('nothing to capture.'); return; }
  const png = chosen.filter((s) => s.kind === 'png');
  const webm = chosen.filter((s) => s.kind === 'webm');
  console.log(`\ncapturing → ${IMG}\n`);
  if (png.length) { console.log('screenshots:'); await runPngShots(png); }
  for (const s of webm) { console.log(`\nclip: ${s.file}`); await runWebmShot(s); }
  console.log('\ndone.');
}

// ──────────────────────────── interactive checkbox ────────────────────────────

function interactiveSelect(items) {
  return new Promise((resolve) => {
    const checked = new Set();
    let cursor = 0;
    readline.emitKeypressEvents(process.stdin);
    if (process.stdin.isTTY) process.stdin.setRawMode(true);
    const render = (first = false) => {
      if (!first) process.stdout.write(`\x1b[${items.length + 3}A`); // move up to redraw
      process.stdout.write('\x1b[0J'); // clear below
      process.stdout.write('  Pick captures  (↑/↓ move · space toggle · a all · enter run · q quit)\n\n');
      items.forEach((it, i) => {
        const box = checked.has(i) ? '◉' : '○';
        const kind = it.kind === 'webm' ? '🎬' : '📸';
        const pointer = i === cursor ? '\x1b[36m❯\x1b[0m' : ' ';
        const line = `${pointer} ${box} ${kind} ${it.id}  \x1b[2m${it.desc}\x1b[0m`;
        process.stdout.write(line + '\n');
      });
      process.stdout.write('\n');
    };
    render(true);
    const onKey = (str, key) => {
      if (!key) return;
      if (key.name === 'up') cursor = (cursor - 1 + items.length) % items.length;
      else if (key.name === 'down') cursor = (cursor + 1) % items.length;
      else if (key.name === 'space') { checked.has(cursor) ? checked.delete(cursor) : checked.add(cursor); }
      else if (str === 'a') { if (checked.size === items.length) checked.clear(); else items.forEach((_, i) => checked.add(i)); }
      else if (key.name === 'return') { cleanup(); return resolve([...checked].map((i) => items[i].id)); }
      else if (str === 'q' || (key.ctrl && key.name === 'c')) { cleanup(); return resolve([]); }
      render();
    };
    const cleanup = () => { process.stdin.removeListener('keypress', onKey); if (process.stdin.isTTY) process.stdin.setRawMode(false); process.stdin.pause(); process.stdout.write('\n'); };
    process.stdin.on('keypress', onKey);
    process.stdin.resume();
  });
}

// ───────────────────────────────── CLI entry ─────────────────────────────────

const argv = process.argv.slice(2);
if (argv.includes('--list')) {
  for (const s of SHOTS) console.log(`${s.kind === 'webm' ? '🎬' : '📸'} ${s.id.padEnd(28)} ${s.desc}`);
} else if (argv.includes('--all')) {
  await run(SHOTS.map((s) => s.id));
} else if (argv.includes('--png')) {
  await run(SHOTS.filter((s) => s.kind === 'png').map((s) => s.id));
} else if (argv.includes('--webm')) {
  await run(SHOTS.filter((s) => s.kind === 'webm').map((s) => s.id));
} else if (argv.length) {
  // tokens may be `id` or `id=<issue id|url>` (for shots that need an issue)
  const ids = [], issueMap = {};
  for (const a of argv) {
    if (a.includes('=')) { const i = a.indexOf('='); const k = a.slice(0, i); ids.push(k); issueMap[k] = a.slice(i + 1); }
    else ids.push(a);
  }
  const unknown = ids.filter((a) => !SHOTS.some((s) => s.id === a));
  if (unknown.length) { console.error(`unknown shot id(s): ${unknown.join(', ')}\nrun --list to see all`); process.exit(1); }
  await run(ids, issueMap);
} else if (process.stdin.isTTY) {
  const ids = await interactiveSelect(SHOTS);
  if (ids.length) await run(ids); else console.log('nothing picked.');
} else {
  console.log('No TTY and no shot ids given. Use --list, --all, --png, --webm, or pass ids. See header.');
}

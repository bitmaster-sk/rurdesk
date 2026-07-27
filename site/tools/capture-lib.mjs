/**
 * Shared capture primitives for the à-la-carte menu (capture-menu.mjs) and the
 * end-to-end scenario runner (scenario.mjs). Everything here is parameterized —
 * no reliance on a single global project/issue — so the scenario can build its
 * own project and thread ids through explicitly.
 */
import path from 'node:path';
import fs from 'node:fs';
import os from 'node:os';
import readline from 'node:readline';
import { execFileSync } from 'node:child_process';
import { chromium } from 'playwright';

export const VW = 1600, VH = 1000;
export const SETTLE_MS = 1800;

export const BASE = (process.env.CAPTURE_BASE || 'http://localhost').replace(/\/+$/, '');
export const IMG = path.resolve(path.dirname(new URL(import.meta.url).pathname), '..', 'assets', 'img');

// ───────────────────────────── browser lifecycle ─────────────────────────────

export async function launch({ video = false } = {}) {
  const browser = await chromium.launch();
  // DSF 2 for video too: the page renders at 2× device px and Playwright downsamples
  // to the recordVideo size (= CSS viewport) → supersampled, crisp text. DSF 1 was
  // the main cause of the pixelated clips. (Heavier to record, but fine for demos.)
  const opts = { viewport: { width: VW, height: VH }, deviceScaleFactor: 2 };
  if (video) opts.recordVideo = { dir: IMG, size: { width: VW, height: VH } };
  const context = await browser.newContext(opts);
  const page = await context.newPage();
  return { browser, context, page };
}

export function onLoginPage(page) {
  return new URL(page.url()).pathname.replace(/\/+$/, '').endsWith('/login');
}

export async function settle(page, ms = SETTLE_MS) { await page.waitForTimeout(ms); }

// log in as a specific user, replacing any current session
export async function loginAs(page, email, password) {
  await page.goto(`${BASE}/login`, { waitUntil: 'domcontentloaded' });
  await page.evaluate(() => localStorage.removeItem('Authorization')).catch(() => {});
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

export async function gotoApp(page, url) {
  await page.goto(url, { waitUntil: 'networkidle' });
  if (onLoginPage(page)) throw new Error(`redirected to /login at ${url} — session lost`);
}

// authenticated JSON fetch against the API using the stored token (for
// idempotency checks + lightweight seeding)
export async function api(page, method, urlPath, body) {
  return await page.evaluate(async ([m, u, b]) => {
    const res = await fetch(u, {
      method: m,
      headers: { Authorization: localStorage.getItem('Authorization'), 'Content-Type': 'application/json' },
      body: b ? JSON.stringify(b) : undefined,
    });
    let data = null; try { data = await res.json(); } catch { /* no body */ }
    return { ok: res.ok, status: res.status, data };
  }, [method, urlPath.startsWith('http') ? urlPath : `${BASE}${urlPath}`, body ?? null]);
}

// ───────────────────────────────── shooting ─────────────────────────────────

export async function shootPage(page, file) {
  if (onLoginPage(page)) throw new Error('on /login — session not authenticated');
  await page.screenshot({ path: path.join(IMG, file) });
}

export async function shootElement(page, sel, file, timeout = 12000) {
  const el = page.locator(sel).first();
  await el.waitFor({ state: 'visible', timeout });
  await el.scrollIntoViewIfNeeded(); await page.waitForTimeout(400);
  await el.screenshot({ path: path.join(IMG, file) });
}

// Like shootElement but caps the captured height. Agent comment cards (plan/design)
// can be tens of thousands of px tall (full diffs) — a raw element screenshot then
// scales down to an illegible ribbon. This scrolls the element's top to the viewport
// top and clips to at most maxHeightCss (and never past the viewport). Short elements
// are captured whole; only over-tall ones get trimmed to their readable top.
export async function shootElementCapped(page, sel, file, maxHeightCss = 900, timeout = 12000) {
  const el = page.locator(sel).first();
  await el.waitFor({ state: 'visible', timeout });
  await el.evaluate((e) => e.scrollIntoView({ block: 'start', inline: 'nearest' }));
  await page.waitForTimeout(400);
  const box = await el.boundingBox();
  if (!box) throw new Error(`no bounding box for ${sel}`);
  const x = Math.max(0, box.x);
  const y = Math.max(0, box.y);
  const offTop = y - box.y; // px of the element scrolled above the viewport top
  const height = Math.min(box.height - offTop, maxHeightCss, VH - y);
  const width = Math.min(box.width, VW - x);
  await page.screenshot({ path: path.join(IMG, file), clip: { x, y, width, height } });
}

// Isolate ONE comment card and shoot it as a single tall, non-scrollable image.
// Agent comment cards (plan/design) live inside the fixed-height, scrollable
// `.feed-scroll` and a narrow flex column — an element screenshot then either
// clips (feed height / diff overflow) or scales to an illegible ribbon. Instead
// we strip the page down to just the target card, release every inner scroll/clip
// clamp, let it grow to its content width, then take a full-page screenshot. The
// result is the WHOLE comment (no vertical cut, no horizontal cut) on a clean bg.
// The DOM is destroyed in the process — fine, each scenario step re-navigates.
// Shared by the isolate-then-shoot helpers below. Strips the page down to ONE card
// on a clean background at a fixed readable width, with every vertical scroll/height
// clamp released so the content renders top to bottom. Horizontal overflow (diffs)
// is deliberately left to scroll inside the card, as in-app, so long code lines
// never spill past the width. Returns the CSS width the card was set to, so the
// caller can size the viewport. Pass `width: null` to keep the card's natural
// width, and `closest: null` to shoot the matched element itself rather than its
// enclosing `.activity-item`.
async function isolateCard(page, sel, { width = null, pad = 20, closest = '.activity-item', stripBorder = false } = {}) {
  const used = await page.evaluate(([sel, width, pad, closest, stripBorder]) => {
    const target = document.querySelector(sel);
    const card = (closest && target.closest(closest)) || target;
    const bg = getComputedStyle(document.body).backgroundColor || '#ffffff';
    const w = width || Math.ceil(card.getBoundingClientRect().width);
    document.body.replaceChildren(card);
    document.documentElement.style.cssText = 'overflow:visible;height:auto;';
    document.body.style.cssText = `overflow:visible;height:auto;margin:0;padding:${pad}px;display:inline-block;background:${bg};`;
    card.style.width = w + 'px';
    card.style.maxWidth = w + 'px';
    if (stripBorder) { card.style.border = 'none'; card.style.borderRadius = '0'; card.style.boxShadow = 'none'; }
    card.querySelectorAll('*').forEach((n) => {
      const s = getComputedStyle(n);
      if (/(auto|scroll)/.test(s.overflowY)) n.style.overflowY = 'visible';
      n.style.maxHeight = 'none';
    });
    // querySelectorAll('*') excludes the card itself, and the card CAN be the
    // scroll clamp (e.g. .quality-panel) — release it too or the shot gets cut.
    card.style.overflowY = 'visible';
    card.style.maxHeight = 'none';
    return w;
  }, [sel, width, pad, closest, stripBorder]);
  await page.waitForTimeout(300);
  return used;
}

export async function shootCommentFull(page, sel, file, { width = 620, pad = 20, timeout = 12000 } = {}) {
  const el = page.locator(sel).first();
  await el.waitFor({ state: 'visible', timeout });
  await isolateCard(page, sel, { width, pad });
  const h = await page.evaluate(() => Math.ceil(document.body.scrollHeight));
  const prev = page.viewportSize() || { width: VW, height: VH };
  await page.setViewportSize({ width: width + pad * 2, height: Math.min(h, 6000) });
  await page.waitForTimeout(200);
  await page.screenshot({ path: path.join(IMG, file), fullPage: true });
  // restore the viewport — otherwise every later screenshot in this run inherits
  // the narrow size and renders stretched/wrong (e.g. the next full-page modal).
  await page.setViewportSize(prev);
  await page.waitForTimeout(150);
}

// Isolate ONE element and shoot it whole, however tall it grows. A plain element
// screenshot clips a panel that lives inside a scrollable container once its
// content outgrows the scroll viewport — `.quality-panel` sits inside
// app-issue-info and does exactly that with many problems + suggestions. Unlike
// the comment helpers this keeps the element's NATURAL width (no fixed reading
// column) and shoots the matched element itself, not an enclosing activity item.
// `stripBorder` drops the outer border/radius/shadow so the capture sits flush.
export async function shootElementFull(page, sel, file, { stripBorder = false, pad = 20, timeout = 12000 } = {}) {
  const el = page.locator(sel).first();
  await el.waitFor({ state: 'visible', timeout });
  const width = await isolateCard(page, sel, { width: null, pad, closest: null, stripBorder });
  const h = await page.evaluate(() => Math.ceil(document.body.scrollHeight));
  const prev = page.viewportSize() || { width: VW, height: VH };
  await page.setViewportSize({ width: width + pad * 2, height: Math.min(h, 6000) });
  await page.waitForTimeout(200);
  await page.screenshot({ path: path.join(IMG, file), fullPage: true });
  await page.setViewportSize(prev);
  await page.waitForTimeout(150);
}

// Like shootCommentFull but keeps only the readable TOP of the card. Agent design
// and implementation-plan comments run to tens of thousands of px (full diffs);
// captured whole they scale down to an illegible ribbon on the docs page.
export async function shootCommentTop(page, sel, file, { width = 620, pad = 20, maxHeight = 900, timeout = 12000 } = {}) {
  const el = page.locator(sel).first();
  await el.waitFor({ state: 'visible', timeout });
  await isolateCard(page, sel, { width, pad });
  const bodyH = await page.evaluate(() => Math.ceil(document.body.scrollHeight));
  const vw = width + pad * 2, h = Math.min(bodyH, maxHeight);
  const prev = page.viewportSize() || { width: VW, height: VH };
  await page.setViewportSize({ width: vw, height: h });
  await page.waitForTimeout(200);
  await page.screenshot({ path: path.join(IMG, file), clip: { x: 0, y: 0, width: vw, height: h } });
  await page.setViewportSize(prev);
  await page.waitForTimeout(150);
}

// Capture the mockup cards of a design comment WITH surrounding context, but not
// the whole (often 2000px+) design prose. The mockup cards sit at the very bottom
// of the comment as small collapsed buttons; a full-comment shot buries them and a
// tight element crop shows "just a button". This isolates the comment (clean bg, no
// feed chrome), then clips a window from `contextAbove` px above the topmost mockup
// down to the comment bottom — so the shot is the mockups plus a slice of design.
export async function shootCommentMockups(page, sel, file, { width = 720, pad = 20, contextAbove = 300, timeout = 12000 } = {}) {
  const el = page.locator(sel).first();
  await el.waitFor({ state: 'visible', timeout });
  await isolateCard(page, sel, { width, pad });
  const prev = page.viewportSize() || { width: VW, height: VH };
  const vw = width + pad * 2;
  const bodyH = await page.evaluate(() => Math.ceil(document.body.scrollHeight));
  // make the whole (now static) comment fit the viewport so an off-viewport clip works
  await page.setViewportSize({ width: vw, height: Math.min(bodyH + pad * 2, 8000) });
  await page.waitForTimeout(150);
  const clipTop = await page.evaluate((ctxAbove) => {
    const mocks = [...document.querySelectorAll('app-mockup-card')];
    if (!mocks.length) return 0;
    const top = Math.min(...mocks.map((m) => m.getBoundingClientRect().top + window.scrollY));
    return Math.max(0, Math.floor(top - ctxAbove));
  }, contextAbove);
  const h = Math.min(bodyH - clipTop, 8000 - clipTop);
  await page.screenshot({ path: path.join(IMG, file), clip: { x: 0, y: clipTop, width: vw, height: h } });
  await page.setViewportSize(prev);
  await page.waitForTimeout(150);
}

// ─────────────────────────────── board / gantt ───────────────────────────────

export async function selectSwimlane(page) {
  const btn = page.getByText('Swimlane', { exact: true }).first();
  if (await btn.count()) { await btn.click(); await page.waitForTimeout(800); }
}

export async function selectGanttDayCompact(page) {
  for (const label of ['Day', 'Compact']) {
    const btn = page.getByText(label, { exact: true }).first();
    if (await btn.count()) { await btn.click(); await page.waitForTimeout(700); }
  }
}

// open a ui-select-style dropdown and click the option with the given text.
// Options render in a CDK overlay (state = <app-state-badge>, severity = .dropdown-item),
// so match by text inside the overlay.
export async function pickDropdownOption(page, dropdownLocator, optionText) {
  await dropdownLocator.scrollIntoViewIfNeeded();
  await dropdownLocator.click(); await page.waitForTimeout(400);
  await page.locator('.cdk-overlay-container').getByText(optionText, { exact: false }).first().click();
  await page.waitForTimeout(400);
}

// ───────────────────────────── injected cursor ─────────────────────────────

export const CURSOR_JS = `(() => {
  if (window.__cur) return;
  const c = document.createElement('div');
  c.id = '__fakecursor';
  Object.assign(c.style, {
    // inset/margin/padding/overflow reset the UA popover defaults (inset:0 + auto
    // margin would centre the dot, the default padding would fatten it). These must
    // come BEFORE left/top — 'inset' is their shorthand and would clobber them.
    position:'fixed', inset:'auto', margin:'0', padding:'0', overflow:'visible',
    zIndex:2147483647, left:'0px', top:'0px',
    width:'20px', height:'20px', borderRadius:'50%', background:'rgba(20,20,20,.85)',
    border:'2px solid #fff', boxShadow:'0 1px 4px rgba(0,0,0,.4)', pointerEvents:'none',
    transform:'translate(-10px,-10px)' });

  // Angular CDK puts its overlay bounding box in the browser TOP LAYER — it sets
  // popover="manual" at runtime (class cdk-overlay-popover, :popover-open). The top
  // layer paints above EVERY z-index in the document, 2147483647 included; measured
  // on /project-builder, a dot at max z-index is invisible inside an open
  // .ui-select-panel and visible one pixel outside it. z-index cannot win this, so
  // the cursor joins the top layer too. There the most recently promoted element is
  // painted last, hence the re-promote on every move.
  const supportsPopover = typeof c.showPopover === 'function';
  if (supportsPopover) c.setAttribute('popover', 'manual');
  document.documentElement.appendChild(c);
  window.__cur = c;

  const raise = () => {
    if (!supportsPopover) { // fallback: last sibling wins the paint tie
      if (document.documentElement.lastElementChild !== c) document.documentElement.appendChild(c);
      return;
    }
    // hide+show is synchronous, so no frame ever renders with the cursor demoted
    try { if (c.matches(':popover-open')) c.hidePopover(); c.showPopover(); } catch { /* ignore */ }
  };
  raise();
  window.__setCur = (x,y) => { c.style.left = x+'px'; c.style.top = y+'px'; raise(); };
})();`;

export function cursorState() {
  const st = { x: VW / 2, y: VH / 2 };
  return {
    async ensure(page) { await page.evaluate(CURSOR_JS); await page.evaluate(([x, y]) => window.__setCur(x, y), [st.x, st.y]); },
    async moveTo(page, x, y, steps = 40) {
      const sx = st.x, sy = st.y;
      for (let i = 1; i <= steps; i++) {
        const t = i / steps;
        const e = t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2;
        const nx = sx + (x - sx) * e, ny = sy + (y - sy) * e;
        await page.mouse.move(nx, ny);
        await page.evaluate(([px, py]) => window.__setCur && window.__setCur(px, py), [nx, ny]);
        await page.waitForTimeout(12);
      }
      st.x = x; st.y = y;
    },
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

export async function centerOf(locator) {
  const b = await locator.boundingBox();
  if (!b) throw new Error('element has no bounding box (not visible?)');
  return [Math.round(b.x + b.width / 2), Math.round(b.y + b.height / 2)];
}

// ───────────────────────────────── ffmpeg ─────────────────────────────────

export function findFfmpeg() {
  const base = path.join(os.homedir(), 'Library', 'Caches', 'ms-playwright');
  try {
    for (const d of fs.readdirSync(base)) {
      if (!d.startsWith('ffmpeg-')) continue;
      const p = path.join(base, d, 'ffmpeg-mac');
      if (fs.existsSync(p)) return p;
    }
  } catch { /* none */ }
  return null;
}

// rename the newest .webm in IMG to `file`, then re-encode: trim `offsetSec` off the
// front (if any) AND raise quality. Playwright's raw VP8 is low-bitrate/blocky; a
// 5000k VP8 pass (bundled ffmpeg has no VP9) preserves the DSF-2 supersampled frames
// far better. Always re-encode so quality is consistent whether or not we trim.
export function finalizeWebm(file, offsetSec = 0) {
  const webms = fs.readdirSync(IMG).filter((f) => f.endsWith('.webm')).map((f) => path.join(IMG, f));
  if (!webms.length) return false;
  const newest = webms.sort((a, b) => fs.statSync(b).mtimeMs - fs.statSync(a).mtimeMs)[0];
  const dst = path.join(IMG, file);
  if (newest !== dst) fs.renameSync(newest, dst);
  const ff = findFfmpeg();
  if (!ff) return true; // no ffmpeg → keep Playwright's raw webm
  const tmp = dst.replace(/\.webm$/, '.enc.webm');
  const args = ['-y'];
  if (offsetSec > 0.05) args.push('-ss', offsetSec.toFixed(2)); // fast front trim
  args.push('-i', dst, '-c:v', 'libvpx', '-b:v', '5000k', '-quality', 'good', '-cpu-used', '1', '-an', tmp);
  execFileSync(ff, args, { stdio: 'ignore' });
  fs.renameSync(tmp, dst);
  return true;
}

// ───────────────────────────────── misc utils ─────────────────────────────────

export function parseIssueRef(input) {
  const s = String(input).trim();
  const m = s.match(/\/issue\/([^/?#]+)/);
  if (m) return m[1];
  return (s.split('?')[0].split('#')[0].replace(/\/+$/, '').split('/').pop() || s);
}

export function promptLine(question) {
  return new Promise((resolve) => {
    const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
    rl.question(question, (a) => { rl.close(); resolve(a.trim()); });
  });
}

export async function raceVisible(locators, timeout) {
  return await Promise.race(
    locators.map((loc, i) => loc.first().waitFor({ state: 'visible', timeout }).then(() => i))
  ).catch(() => -1);
}

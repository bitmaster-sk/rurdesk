/**
 * Behavior tests for the site build: pages exist, links resolve, missing
 * assets degrade to placeholders, present assets render as media.
 */
import { test, before } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { marked } from 'marked';
import { build, SIDEBAR, NAV, wrapTables } from '../tools/build.mjs';

const SITE = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const DIST = path.join(SITE, 'dist');

let result;

before(() => {
    result = build();
});

const read = (rel) => fs.readFileSync(path.join(DIST, rel), 'utf8');

test('every sidebar page is generated', () => {
    for (const group of SIDEBAR) {
        for (const page of group.pages) {
            assert.ok(fs.existsSync(path.join(DIST, 'docs', page.out)), `missing dist/docs/${page.out}`);
        }
    }
});

test('roadmap is not generated or linked from the site', () => {
    assert.ok(!fs.existsSync(path.join(DIST, 'docs', 'roadmap.html')), 'roadmap.html should not be built');
    const landing = read('index.html');
    assert.ok(!/href="[^"]*roadmap\.html"/.test(landing), 'landing must not link to roadmap.html');
    for (const group of SIDEBAR) {
        for (const page of group.pages) {
            const html = read(path.join('docs', page.out));
            assert.ok(!/href="[^"]*roadmap\.html"/.test(html), `${page.out} must not link to roadmap.html`);
        }
    }
});

test('landing page and assets are copied to dist', () => {
    assert.ok(fs.existsSync(path.join(DIST, 'index.html')));
    assert.ok(fs.existsSync(path.join(DIST, 'assets', 'css', 'site.css')));
    assert.ok(fs.existsSync(path.join(DIST, 'assets', 'img', 'architecture.svg')));
});

test('no generated page links to a .md file', () => {
    for (const group of SIDEBAR) {
        for (const page of group.pages) {
            const html = read(path.join('docs', page.out));
            const mdLinks = html.match(/href="[^"]+\.md[^"]*"/g) || [];
            assert.deepEqual(mdLinks, [], `${page.out} still links to markdown: ${mdLinks}`);
        }
    }
});

test('internal doc links point at pages that exist', () => {
    for (const group of SIDEBAR) {
        for (const page of group.pages) {
            const html = read(path.join('docs', page.out));
            for (const match of html.matchAll(/href="\.\/([\w-]+\.html)/g)) {
                assert.ok(
                    fs.existsSync(path.join(DIST, 'docs', match[1])),
                    `${page.out} links to missing ${match[1]}`
                );
            }
        }
    }
});

test('missing assets render as placeholders with the expected filename', () => {
    // hero.png does not exist until the user captures it
    if (!fs.existsSync(path.join(SITE, 'assets', 'img', 'hero.png'))) {
        const landing = read('index.html');
        assert.match(landing, /asset-placeholder/, 'landing should contain placeholders');
        assert.match(landing, /data-asset="hero\.png"/, 'placeholder should name the missing file');
        assert.ok(result.missingAssets.includes('hero.png'));
    }
});

test('present assets render as real media, not placeholders', () => {
    // architecture.svg ships with the repo and is referenced from the guide index
    const overview = read(path.join('docs', 'index.html'));
    assert.match(overview, /<img src="\.\.\/assets\/img\/architecture\.svg"/);
    assert.doesNotMatch(overview, /data-asset="architecture\.svg"/);
});

test('video references become <video> or video placeholders, never <img>', () => {
    const features = read(path.join('docs', 'features.html'));
    const webmImgs = features.match(/<img[^>]+\.webm/g) || [];
    assert.deepEqual(webmImgs, [], 'webm must not render as <img>');
    assert.ok(
        /asset-placeholder--video|<video[^>]+\.webm/.test(features),
        'webm should render as video or video placeholder'
    );
});

test('sidebar of every page contains all nav entries and marks itself active', () => {
    const navLabels = SIDEBAR.flatMap((g) => g.pages.map((p) => p.nav));
    for (const group of SIDEBAR) {
        for (const page of group.pages) {
            const html = read(path.join('docs', page.out));
            for (const label of navLabels) {
                assert.ok(html.includes(`>${label}</a>`), `${page.out} sidebar misses "${label}"`);
            }
            assert.ok(
                html.includes(`class="active" href="./${page.out}"`),
                `${page.out} does not mark itself active`
            );
        }
    }
});

test('landing and docs top bars offer the same navigation', () => {
    const topbarNav = (html) => {
        const nav = html.match(/<nav class="topbar__nav">([\s\S]*?)<\/nav>/);
        assert.ok(nav, 'page has no topbar nav');
        return [...nav[1].matchAll(/>([^<>]+)<\/a>/g)].map((m) => m[1].trim());
    };

    const expected = NAV.map((item) => item.label);
    assert.deepEqual(topbarNav(read('index.html')), expected, 'landing nav drifted from NAV');
    for (const group of SIDEBAR) {
        for (const page of group.pages) {
            assert.deepEqual(
                topbarNav(read(path.join('docs', page.out))),
                expected,
                `docs/${page.out} nav drifted from NAV`
            );
        }
    }
});

test('docs pages link back to landing sections, landing uses bare fragments', () => {
    const anchors = NAV.filter((item) => item.anchor);

    const landing = read('index.html');
    for (const item of anchors) {
        assert.ok(
            landing.includes(`href="#${item.anchor}"`),
            `landing should link "${item.label}" as a same-page fragment`
        );
        assert.ok(
            landing.includes(`id="${item.anchor}"`),
            `landing has no section with id="${item.anchor}"`
        );
    }

    const overview = read(path.join('docs', 'index.html'));
    for (const item of anchors) {
        assert.ok(
            overview.includes(`href="../index.html#${item.anchor}"`),
            `docs should link "${item.label}" back to the landing page`
        );
    }
});

test('docs top bar marks Docs as the current section', () => {
    const overview = read(path.join('docs', 'index.html'));
    assert.match(overview, /<a class="active" href="\.\.\/docs\/index\.html">Docs<\/a>/);
});

test('headings get anchor ids so cross-page #links work', () => {
    const features = read(path.join('docs', 'features.html'));
    assert.match(features, /<h3 id="project-kickstarter">/);
});

test('a stackTables page labels every cell with its column header', () => {
    const config = read(path.join('docs', 'configuration.html'));
    assert.match(config, /<div class="table-scroll table-scroll--stack">/);
    // Cells carry their header so CSS can render them as key/value cards.
    assert.match(config, /<td data-label="Example" role="cell"><span class="cell"><code>5432<\/code><\/span><\/td>/);
    assert.match(config, /<td data-label="Purpose" role="cell"><span class="cell">Postgres port<\/span><\/td>/);
    // Inline markup stays inside one wrapper, or the card grid scatters it.
    assert.match(config, /<td data-label="Purpose" role="cell"><span class="cell">One of <code>anthropic<\/code>/);
    // display:block would strip the table semantics, so the roles are pinned.
    assert.match(config, /<table role="table">/);
    assert.match(config, /<tr role="row">/);
});

test('tables stay scrollable on pages that did not opt in', () => {
    const users = read(path.join('docs', 'user-management.html'));
    assert.ok(users.includes('<div class="table-scroll">'), 'expected a plain scroll wrapper');
    assert.ok(!users.includes('table-scroll--stack'), 'comparison matrices must not stack');
    assert.ok(!users.includes('data-label'), 'non-opted pages must not get cell labels');
});

test('a single table opts in with a stack-table comment', () => {
    const html = marked.parse('<!-- stack-table -->\n\n| Host | Token |\n| --- | --- |\n| GitHub | PAT |\n\n| Plain | Table |\n| --- | --- |\n| a | b |\n');
    const out = wrapTables(html);

    assert.match(out, /<div class="table-scroll table-scroll--stack">[\s\S]*?<td data-label="Token" role="cell"><span class="cell">PAT<\/span><\/td>/);
    // The marker is consumed by the wrapper, not left dangling in the output.
    assert.ok(!out.includes('stack-table'), 'marker comment should not survive into the HTML');
    // Only the marked table stacks; the next one is untouched.
    assert.match(out, /<div class="table-scroll"><table>[\s\S]*?<td>b<\/td>/);
});

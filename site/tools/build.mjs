/**
 * Static site builder.
 *
 * Sources:
 *   - ./content/<section>/*.md (presentation pages)   → dist/docs/*.html
 *   - ./index.html (hand-written landing page)        → dist/index.html
 *   - ./assets/**                                     → dist/assets/**
 *
 * Media convention: every markdown image reference is resolved by *basename*
 * against site/assets/img/. Missing assets render as styled placeholders so
 * the site is reviewable before screenshots/clips exist.
 * .webm/.mp4 references become looping <video> elements.
 */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { marked } from 'marked';

const SITE = path.join(path.dirname(fileURLToPath(import.meta.url)), '..');
const BASE_URL = 'https://rurdesk.com';
const CONTENT = path.join(SITE, 'content');
const DIST = path.join(SITE, 'dist');
const IMG_DIR = path.join(SITE, 'assets', 'img');

export const SIDEBAR = [
    {
        section: 'Getting started',
        pages: [
            { src: 'getting-started/index.md', out: 'index.html', nav: 'Overview' },
            { src: 'getting-started/installation.md', out: 'installation.html', nav: 'Installation & first run' },
            { src: 'getting-started/configuration.md', out: 'configuration.html', nav: 'Configuration' }
        ]
    },
    {
        section: 'Using the tracker',
        pages: [
            { src: 'using-the-tracker/user-management.md', out: 'user-management.html', nav: 'User management' },
            { src: 'using-the-tracker/features.md', out: 'features.html', nav: 'Projects, tasks & AI' },
            { src: 'using-the-tracker/sprints.md', out: 'sprints.html', nav: 'Sprints' },
            { src: 'using-the-tracker/saved-views.md', out: 'saved-views.html', nav: 'Saved views' },
            { src: 'using-the-tracker/git-integration.md', out: 'git-integration.html', nav: 'Git integration' }
        ]
    },
    {
        section: 'Agentic workflow',
        pages: [
            { src: 'agentic-workflow/agents.md', out: 'agents.html', nav: 'Bots & agent runs' },
            { src: 'agentic-workflow/gateway.md', out: 'gateway.html', nav: 'Agent gateway' }
        ]
    }
];

/**
 * Top-bar navigation — the single source of truth for both the landing page
 * and the docs layout, injected as {{NAV}}. Keeping it here is what stops the
 * two menus from drifting apart.
 *
 * `anchor` items point at a landing-page section: a bare fragment on the
 * landing page itself, a full path from anywhere else. `path` items are
 * plain links relative to the dist root.
 */
export const NAV = [
    { label: 'Features', anchor: 'features' },
    { label: 'Agents', anchor: 'agents' },
    { label: 'Self-hosted', anchor: 'self-hosted' },
    { label: 'Docs', path: 'docs/index.html', key: 'docs' },
    { label: 'Get started', path: 'docs/installation.html', cta: true }
];

/**
 * @param root  '' on the landing page, '../' on a docs page.
 * @param activeKey  key of the nav item to mark as current, if any.
 */
export function navHtml(root, activeKey = null) {
    const links = NAV.map((item) => {
        const href = item.anchor
            ? (root === '' ? `#${item.anchor}` : `${root}index.html#${item.anchor}`)
            : `${root}${item.path}`;
        let cls = '';
        if (item.cta) {
            cls = ' class="btn btn--primary"';
        } else if (item.key && item.key === activeKey) {
            cls = ' class="active"';
        }
        return `        <a${cls} href="${href}">${item.label}</a>`;
    });
    return `<nav class="topbar__nav">\n${links.join('\n')}\n    </nav>`;
}

const missingAssets = new Set();

function parseFrontmatter(text) {
    const match = text.match(/^---\n([\s\S]*?)\n---\n/);
    if (!match) {
        return { meta: {}, body: text };
    }
    const meta = {};
    for (const line of match[1].split('\n')) {
        const kv = line.match(/^(\w+):\s*(.*)$/);
        if (kv) {
            meta[kv[1]] = kv[2].trim();
        }
    }
    return { meta, body: text.slice(match[0].length) };
}

function slugify(text) {
    return text
        .toLowerCase()
        .replace(/<[^>]+>/g, '')
        .replace(/&[a-z]+;/g, '')
        .replace(/[^a-z0-9\s-]/g, '')
        .trim()
        .replace(/\s+/g, '-');
}

function escapeHtml(text) {
    return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

/** Renders an image/video/placeholder for an asset referenced by any path. */
export function mediaHtml(src, alt, imgPrefix, imgDir = IMG_DIR) {
    const base = path.basename(src.split('#')[0].split('?')[0]);
    const exists = fs.existsSync(path.join(imgDir, base));
    const isVideo = /\.(webm|mp4)$/i.test(base);
    const caption = alt ? `<figcaption>${escapeHtml(alt)}</figcaption>` : '';
    if (!exists) {
        missingAssets.add(base);
        const icon = isVideo ? '▶' : '◻';
        return `<div class="asset-placeholder${isVideo ? ' asset-placeholder--video' : ''}" data-asset="${escapeHtml(base)}">` +
            `<span class="asset-placeholder__icon">${icon}</span>` +
            `<span class="asset-placeholder__label">${escapeHtml(alt || base)}</span>` +
            `<span class="asset-placeholder__file">${escapeHtml(base)}</span></div>`;
    }
    // data-zoom opts the element into the lightbox (assets/js/lightbox.js).
    const zoom = `data-zoom tabindex="0" role="button" aria-label="Enlarge: ${escapeHtml(alt || base)}"`;
    if (isVideo) {
        return `<figure class="doc-figure"><video src="${imgPrefix}${base}" autoplay loop muted playsinline ${zoom}></video>${caption}</figure>`;
    }
    return `<figure class="doc-figure"><img src="${imgPrefix}${base}" alt="${escapeHtml(alt)}" loading="lazy" ${zoom}>${caption}</figure>`;
}

/** Post-processing shared by docs pages and the landing page. */
function processImages(html, imgPrefix) {
    return html.replace(/<img\b[^>]*>/g, (tag) => {
        const src = (tag.match(/src="([^"]*)"/) || [])[1];
        if (!src) {
            return tag;
        }
        const alt = (tag.match(/alt="([^"]*)"/) || [])[1] || '';
        return mediaHtml(src, alt, imgPrefix);
    });
}

function processLandingMedia(html, imgPrefix) {
    // Only elements opting in via data-media are checked/replaced.
    return html.replace(/<(img|video)\b[^>]*\bdata-media\b[^>]*>(?:<\/video>)?/g, (tag) => {
        const src = (tag.match(/src="([^"]*)"/) || [])[1];
        const alt = (tag.match(/(?:alt|data-label)="([^"]*)"/) || [])[1] || '';
        if (!src) {
            return tag;
        }
        return mediaHtml(src, alt, imgPrefix);
    });
}

function rewriteMarkdownLinks(html) {
    // ./installation.md, ../roadmap.md, ../user-guide/x.md#anchor → flat sibling .html
    return html.replace(/href="([^"]+\.md)(#[^"]*)?"/g, (whole, file, anchor = '') => {
        if (/^[a-z]+:\/\//.test(file)) {
            return whole;
        }
        const base = path.basename(file, '.md');
        return `href="./${base}.html${anchor}"`;
    });
}

/**
 * Tables get their own scroll container so a wide one never widens the page.
 *
 * A table may additionally opt into the **stacked** mobile layout, where each
 * row collapses into a key/value card (column header : cell) instead of
 * scrolling sideways. Opt in per page with `stackTables: true` in the
 * frontmatter, or per table with a `<!-- stack-table -->` comment right above
 * it in the markdown.
 *
 * Stacking suits tables whose rows are independent entities and whose first
 * column identifies the row (it becomes the card title). It is a poor fit for
 * comparison matrices, whose value lies in reading columns side by side — those
 * are better left scrolling.
 */
export function wrapTables(html, stackAll = false) {
    return html.replace(/(<!--\s*stack-table\s*-->\s*)?<table>[\s\S]*?<\/table>/g, (whole, marker) => {
        const table = marker ? whole.slice(marker.length) : whole;
        if (!stackAll && !marker) {
            return `<div class="table-scroll">${table}</div>`;
        }
        return `<div class="table-scroll table-scroll--stack">${stackTable(table)}</div>`;
    });
}

/**
 * Labels every cell with its column header (`data-label`) so CSS can render it
 * as a key/value card, and pins the ARIA table roles that `display: block`
 * would otherwise strip from the element.
 *
 * The cell body is wrapped in a `<span class="cell">` because the card layout
 * makes the `<td>` a grid: without the wrapper, inline children (`<code>`,
 * `<strong>`, …) each become their own grid item and get scattered across the
 * label/value columns.
 */
function stackTable(table) {
    const headers = [...table.matchAll(/<th[^>]*>([\s\S]*?)<\/th>/g)]
        .map((match) => match[1].replace(/<[^>]+>/g, '').trim());
    let labelled = table.replace(/<tr>([\s\S]*?)<\/tr>/g, (row, cells) => {
        let column = 0;
        return `<tr>${cells.replace(/<td([^>]*)>([\s\S]*?)<\/td>/g, (cell, attrs, body) => {
            const label = headers[column++] || '';
            return `<td${attrs} data-label="${escapeHtml(label)}"><span class="cell">${body}</span></td>`;
        })}</tr>`;
    });
    labelled = labelled
        .replace('<table>', '<table role="table">')
        .replace(/<(thead|tbody)>/g, '<$1 role="rowgroup">')
        .replace(/<tr>/g, '<tr role="row">')
        .replace(/<th([^>]*)>/g, '<th$1 role="columnheader">')
        .replace(/<td([^>]*)>/g, '<td$1 role="cell">');
    return labelled;
}

function addHeadingIds(html) {
    return html.replace(/<h([23])>([\s\S]*?)<\/h\1>/g, (whole, level, inner) =>
        `<h${level} id="${slugify(inner)}">${inner}</h${level}>`);
}

function sidebarHtml(activeOut) {
    const groups = SIDEBAR.map((group) => {
        const links = group.pages.map((page) => {
            const active = page.out === activeOut ? ' class="active"' : '';
            return `<li><a${active} href="./${page.out}">${page.nav}</a></li>`;
        }).join('\n');
        return `<div class="sidebar__group"><div class="sidebar__section">${group.section}</div><ul>\n${links}\n</ul></div>`;
    });
    return groups.join('\n');
}

export function build() {
    missingAssets.clear();
    const layout = fs.readFileSync(path.join(SITE, 'template', 'layout.html'), 'utf8');

    fs.rmSync(DIST, { recursive: true, force: true });
    fs.mkdirSync(path.join(DIST, 'docs'), { recursive: true });
    fs.cpSync(path.join(SITE, 'assets'), path.join(DIST, 'assets'), { recursive: true });

    // Docs pages
    for (const group of SIDEBAR) {
        for (const page of group.pages) {
            const raw = fs.readFileSync(path.join(CONTENT, page.src), 'utf8');
            const { meta, body } = parseFrontmatter(raw);
            let html = marked.parse(body);
            html = rewriteMarkdownLinks(html);
            html = processImages(html, '../assets/img/');
            html = wrapTables(html, meta.stackTables === 'true');
            html = addHeadingIds(html);
            const title = meta.title || page.nav;
            const out = layout
                .replaceAll('{{ROOT}}', '../')
                .replaceAll('{{BASE_URL}}', BASE_URL)
                .replaceAll('{{CANONICAL}}', `${BASE_URL}/docs/${page.out}`)
                .replaceAll('{{TITLE}}', escapeHtml(title))
                .replaceAll('{{DESCRIPTION}}', escapeHtml(meta.description || ''))
                .replaceAll('{{NAV}}', navHtml('../', 'docs'))
                .replaceAll('{{SIDEBAR}}', sidebarHtml(page.out))
                .replaceAll('{{CONTENT}}', html);
            fs.writeFileSync(path.join(DIST, 'docs', page.out), out);
        }
    }

    // Landing page
    let landing = fs.readFileSync(path.join(SITE, 'index.html'), 'utf8');
    landing = landing.replaceAll('{{BASE_URL}}', BASE_URL);
    landing = landing.replaceAll('{{NAV}}', navHtml(''));
    landing = processLandingMedia(landing, 'assets/img/');
    fs.writeFileSync(path.join(DIST, 'index.html'), landing);

    // sitemap.xml + robots.txt
    const urls = [
        `${BASE_URL}/`,
        ...SIDEBAR.flatMap((group) => group.pages.map((page) => `${BASE_URL}/docs/${page.out}`))
    ];
    const sitemap = '<?xml version="1.0" encoding="UTF-8"?>\n' +
        '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n' +
        urls.map((url) => `  <url><loc>${url}</loc></url>`).join('\n') +
        '\n</urlset>\n';
    fs.writeFileSync(path.join(DIST, 'sitemap.xml'), sitemap);
    fs.writeFileSync(path.join(DIST, 'robots.txt'),
        `User-agent: *\nAllow: /\n\nSitemap: ${BASE_URL}/sitemap.xml\n`);

    return { missingAssets: [...missingAssets].sort() };
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
    const { missingAssets: missing } = build();
    console.log(`built → ${path.relative(process.cwd(), DIST)}`);
    if (missing.length > 0) {
        console.log(`\n${missing.length} asset(s) still rendered as placeholders:`);
        for (const name of missing) {
            console.log(`  - assets/img/${name}`);
        }
        console.log('\nSee site/TASKS.md for how to capture them.');
    } else {
        console.log('all assets present — no placeholders.');
    }
}

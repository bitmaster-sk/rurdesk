# Presentation page + docs site

Static site for the tracker: a hand-crafted landing page (`index.html`) and docs
generated from the markdown in `content/` (organized by site section).
**The markdown stays the single source of truth — edit text there, never in
`dist/`.**

## Build

```bash
cd site
npm install          # once — installs marked
node tools/build.mjs # → dist/
```

Open `dist/index.html` in a browser (works straight from disk, no server).

The build prints every screenshot/clip still missing — those render as styled
placeholders, so the site is fully reviewable before assets exist.

## Layout

```
site/
  index.html          landing page source (edit directly)
  content/            docs markdown, organized by site section (getting-started/, …)
  template/layout.html  shared docs shell (header, sidebar, footer)
  assets/css/site.css   all styling (palette mirrors client/src/app/theme/app-preset.ts)
  assets/img/           screenshots, .webm clips, architecture.svg, favicon
  tools/build.mjs       md → HTML generator (page list + sidebar defined here)
  tools/capture.mjs     automated Playwright screenshots
  test/                 build behavior tests
  dist/                 output — never edit, always regenerate
```

## Editing content

| What | Where |
| --- | --- |
| Docs text | `content/<section>/*.md` |
| Landing page copy/sections | `index.html` |
| Add a docs page | drop the `.md` in `content/<section>/`, register it in `SIDEBAR` in `tools/build.mjs` |
| Styling | `assets/css/site.css` |

Media convention in markdown: a normal image reference whose filename exists in
`site/assets/img/` — matched **by basename**, so any relative path works.
`.webm`/`.mp4` files become autoplaying looped videos. Missing files become
placeholders showing the expected filename.

## Tables on mobile

By default a wide table scrolls sideways inside its own container. A table can
instead opt into the **stacked** layout, where below 640px each row becomes a
key/value card (column header : cell value) and nothing scrolls:

| Scope | How |
| --- | --- |
| Whole page | `stackTables: true` in the markdown frontmatter |
| One table | `<!-- stack-table -->` on its own line right above the table |

Stack tables whose rows are independent entities and whose **first column
identifies the row** — it becomes the card's title. Leave comparison matrices
(e.g. the role/capability grid in `user-management.md`) scrolling: their value
is in reading the columns side by side, which cards destroy.

## Test

```bash
npm test             # node --test — verifies build output behavior
```

## Screenshots / clips

```bash
npm i && npx playwright install chromium
CAPTURE_EMAIL=you@example.com CAPTURE_PASSWORD=... CAPTURE_PROJECT=1 npm run capture
```

## Publishing (later)

Everything in `dist/` is relative-path static HTML — copy it to any static
host or subpath as-is.

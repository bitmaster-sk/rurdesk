# Contributing

Thanks for taking the time. This file covers how to get the stack running, what
CI expects, and the conventions the codebase already follows.

## Getting the stack up

The quick start in [`README.md`](README.md) is the short version: copy each
`.env.example` to `.env`, build the gateway image with
`./script/build-gateway.sh`, then `docker compose up`.

Per-component setup lives in [`api/README.md`](api/README.md),
[`client/README.md`](client/README.md), [`gateway/README.md`](gateway/README.md)
and [`site/README.md`](site/README.md).

## Before you open a pull request

Run what CI runs. It gates every push and PR
([`.github/workflows/ci.yml`](.github/workflows/ci.yml)).

**Backend** — needs cgo and an isolated test database. `TestMain` refuses to
start unless `TEST_DATABASE_*` is set and the database name contains `"test"`, so
it can never touch a real one. `TEST_CACHE_DB` must be non-zero for the same
reason.

Create the database once; the suite migrates it itself on every run, so an empty
one is a fine starting point:

```bash
createdb -U rurdesk rurdesk_test     # or: docker exec rurdesk_db createdb -U rurdesk rurdesk_test
```

```bash
cd api
go vet ./...
CGO_ENABLED=1 TEST_DATABASE_HOST=localhost TEST_DATABASE_NAME=rurdesk_test \
  TEST_DATABASE_USER=rurdesk TEST_DATABASE_PASSWORD=rurdesk \
  TEST_CACHE_HOST=localhost TEST_CACHE_DB=1 \
  go test ./... -race
gofmt -l .          # must print nothing
```

**Frontend**

```bash
cd client
npm run test:unit   # Vitest, node env, *.spec.ts
npm run test:ct       # Vitest browser (Chromium), *.browser.spec.ts
npm run format:check  # must pass; `npm run format` rewrites
```

Both formatters are CI gates. Run `npm run format` (client) and `gofmt -w .`
(api, gateway) before pushing rather than arguing with the config — nothing in
`.prettierrc` is worth a discussion.

**Gateway**

```bash
cd gateway
go vet ./... && go test ./... && gofmt -l .
```

End-to-end tests are not part of CI on every push; they run nightly against a
production-image stack. Run them yourself with `npm run e2e:stack`, which owns
the whole lifecycle. Never point them at a stack that already has users —
`onboarding.spec.ts` registers the bootstrap user, and public registration closes
permanently once any user exists.

## Conventions

- **Go** — errors wrapped with context (`fmt.Errorf("doing x: %w", err)`), narrow
  interfaces at the point of use, table-driven tests with `t.Run`. Repositories
  write SQL directly; there is no ORM.
- **Angular** — NgModules, not standalone components. `inject()` over constructor
  injection, signals for local state, `OnPush` everywhere, typed reactive forms,
  `@if`/`@for` over the old structural directives.
- **Styling** — Tailwind utilities for layout, `--ui-*` design tokens for colour.
  The `ui-*` components in `src/app/ui/` are the only component library; do not
  add another.
- **Strings** — everything user-facing goes through ngx-translate.

## Tests

Tests verify behaviour, not internals. A test that mirrors the implementation
line by line will pass while the feature is broken, so prefer asserting what a
user or caller observes. Every change ships with its tests.

Coverage is reported in each CI run's job summary. Read it to find untested
areas — it is not a target, and a high percentage of implementation-coupled tests
is worse than fewer honest ones.

## Comments

Explain *why*, not *what*. A comment earns its place by recording a constraint,
a trade-off, or a trap that the code cannot state itself. Comments that restate
the line below them, or narrate how the code used to look, are noise.

## Documentation

`/site` is the user-facing documentation, built from Markdown in
`site/content/`. If a change alters observable behaviour, configuration or
deployment, update it in the same pull request and rebuild with
`node site/tools/build.mjs`. Internal notes belong in the per-component
`README.md` files.

## Reporting bugs and vulnerabilities

Bugs: open an issue with the version or commit, how you deployed, and steps to
reproduce.

Security issues: do **not** open a public issue — see
[`SECURITY.md`](SECURITY.md).

## Licence

Contributions are accepted under the AGPL-3.0 of this repository
([`LICENSE`](LICENSE)).

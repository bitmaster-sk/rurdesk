# issue-client

Angular frontend for the issue tracker.

## Stack

Angular 21 · custom `ui-*` design system (`src/app/ui/`, CDK Overlay) ·
Tailwind CSS v4 · Tabler Icons · ngx-translate · FullCalendar. Uses NgModules
(not standalone components).

## Development server

```bash
npm start
```

Serves on <http://localhost:1000> with live reload. In the full Docker stack
the app is reached through the nginx proxy on <http://localhost>.

## Build

```bash
ng build                                   # default config (production)
ng build --configuration development       # dev build
```

In production there is no separate client image. The single tracker image
([`docker/tracker/Dockerfile.prod`](../docker/tracker/Dockerfile.prod)) builds
the SPA with `ng build --configuration production` and the Go binary serves the
compiled assets. `production` is the `defaultConfiguration` in `angular.json`.

## Testing

Three layers, all on **Vitest** (unit + component) and **Playwright** (e2e).
Karma is gone — there is no `ng test`.

```bash
npm run test:unit   # Vitest node unit tests (*.spec.ts)
npm run test:ct     # Vitest browser component tests, Chromium (*.browser.spec.ts)
npm run e2e:stack   # Playwright against a fresh, throwaway stack
```

### E2E needs a _fresh_ stack — not the dev one

`onboarding.spec.ts` registers the bootstrap (first) user, and public
registration closes permanently once any user exists. Running the suite against
the shared dev stack therefore fails: that instance was bootstrapped by somebody
else, so the login never succeeds. `ensureBootstrapUser` detects exactly this and
fails with the reason instead of a misleading "element not found".

Use `npm run e2e:stack` ([`scripts/e2e-stack.mjs`](scripts/e2e-stack.mjs)). It
brings up the isolated stack
([`docker/e2e/docker-compose.e2e.yml`](../docker/e2e/docker-compose.e2e.yml) —
the production single image plus its own Postgres and Redis), waits for it to
serve, runs the tests, and always tears it down with `-v` so the next run starts
virgin. It needs host port 1000 free. Extra args are forwarded:

```bash
npm run e2e:stack -- --headed
```

The `login.spec.ts` journey creates a fresh user via the admin API, logs in
through the public login form, and asserts the authenticated app shell is
rendered. Other specs cover onboarding, issue lifecycle, and the command palette.

Bare `npm run e2e` starts only the Angular dev server (no backend); it is useful
solely when you point `E2E_BASE_URL` at a stack you manage yourself.

## Project structure

Feature modules under `src/app/`: `auth`, `project`, `issue`, `message`,
`team`, `user`, `core`, `shared`. Each follows the same layout
(`api/`, `components/`, `pages/`, `service/`, `store/`, `model/`, `entity/`,
`resolvers/`). The `issue` module provides four views: table, kanban, calendar
(FullCalendar), and gantt.

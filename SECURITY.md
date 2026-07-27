# Security Policy

## Reporting a vulnerability

Please report security issues privately, not through public issues or pull
requests.

- **Preferred:** GitHub's private vulnerability reporting — *Security* →
  *Report a vulnerability* on this repository.
- **Email:** <info@bitmaster.sk>

You should get an acknowledgement within a few working days. Rurdesk is
maintained by a small team, so please allow reasonable time for a fix before
disclosing publicly. Credit is given in the release notes unless you prefer
otherwise.

Useful things to include: affected version or commit, how the instance is
deployed, reproduction steps, and what an attacker gains.

## Supported versions

Rurdesk's first public release is `v1.0.0`. Security fixes land on the latest
`1.x` release only — there are no maintained older branches or backports, so
upgrade to the newest tag to pick one up.

## Scope

In scope: the API, the Angular client, the MCP server and the agent gateway in
this repository, plus the published container images.

Out of scope: findings that require an already-compromised host or database,
issues in third-party dependencies without a Rurdesk-specific exploit path, and
the deliberate design choices listed below.

## Known design choices

These are intentional and documented, so you do not need to report them — but do
report a concrete exploit that builds on one.

- **Session tokens live in `localStorage`.** They are opaque random values
  resolved against Redis, not signed JWTs, and are deliberately not mirrored into
  a cookie: the WebSocket needs to read the token, and an earlier cookie copy
  drifted out of sync and broke reconnects. The trade-off is that an XSS on the
  origin can read a session, so the client treats XSS as the boundary to defend.
- **Client-side permission checks are UX only.** Route guards and hidden buttons
  are convenience; every mutation is authorized server-side.
- **`Access-Control-Allow-Origin` defaults to `*`.** Authentication is a bearer
  token, never a cookie, so a wildcard does not enable credentialed
  cross-origin requests. Set `ALLOWED_ORIGINS` to restrict it.

## Operating Rurdesk safely

- Give `GIT_ACCESS_TOKEN` the narrowest scope that works — contents read/write on
  the one repository the gateway serves. It reaches the agent's environment.
- Keep `GIT_INTEGRATION_ENCRYPTION_KEY` out of version control and rotate it with
  `admin rotate-git-key`.
- Agents execute code. Run each gateway in its own container with only the
  repository it needs.

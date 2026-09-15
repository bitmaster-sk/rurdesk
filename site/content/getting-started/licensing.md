---
title: Licensing
description: How the free and commercial builds differ, and how to install a license key.
stackTables: true
---

# Licensing

rurdesk ships as **two images from one repository**. The free image is the whole
product under AGPL-3.0. The commercial image adds paid features and needs a
license key.

| Image | Contents | License |
| --- | --- | --- |
| `ghcr.io/<owner>/rurdesk/rurdesk` | Everything except paid features | AGPL-3.0 |
| `ghcr.io/<owner>/rurdesk/rurdesk-commercial` | Adds paid features, gated by a key | AGPL-3.0 core + commercial license for `api/commercial/` |

The free image **contains no paid code at all** — the two images are built from
different entrypoint packages, so this is a property of the artifact, not a
runtime switch. A free instance also never calls out: there is no license
server, no phone-home, no telemetry.

## Running the free build

Nothing to configure. Use the `rurdesk` image and ignore this page.

## Installing a license key

The commercial image reads two environment variables:

| Variable | Example | Purpose |
| --- | --- | --- |
| `LICENSE_KEY` | `eyJ0ZW5h….3Ryb25n` | Your signed license key, as issued |
| `LICENSE_PUBLIC_KEY` | `<base64-32-bytes>` | Ed25519 public key the key is verified against, shipped with your license |

```env
LICENSE_KEY=<your-key>
LICENSE_PUBLIC_KEY=<public-key>
```

Verification is a signature check performed **offline, once at startup**. The
key states the tenant, the plan, the entitled features and an expiry date.

**A missing or invalid key is not a boot failure.** The instance starts, logs
the reason, and runs with paid features off:

```
license: no key configured, paid features are off
license: key rejected (license key signature is not valid), paid features are off
```

## What happens as expiry approaches

Instance admins see a banner in the app. It escalates on its own:

| Time left | Severity | Can be hidden |
| --- | --- | --- |
| 30 days or more | no banner | — |
| under 30 days | info | yes, until the next day |
| under 20 days | warning | yes, until the next day |
| under 10 days | error | no |
| expired | expired | no |

Everyone signed in sees it. The person who can renew the license is often not
the person using the app every day, so restricting the banner to admins would
mean nobody notices until it is too late.

## What happens after expiry

Paid features stop working. That is all that stops.

> **Your data is never held hostage.** Expiry never blocks access to your own
> data, and it never blocks local sign-in. Log in, read, edit and export
> everything exactly as before.

**One thing to check in advance.** If your instance signs in through a paid
authentication path, keep **at least one admin with a local password**. When the
license expires that sign-in path stops working, and without a local admin
nobody can get in on the day it lapses. The banner says so from the `error` step
onward.

## Renewing

Replace `LICENSE_KEY` with the new key and restart:

```bash
docker compose up -d --force-recreate rurdesk.api
```

The banner disappears on the next page load.

---
title: Single sign-on
description: Sign in with your company identity provider over OpenID Connect or SAML 2.0.
---

# Single sign-on

Single sign-on (SSO) lets people sign in with their company account from an
identity provider (IdP) such as Microsoft Entra ID, Okta, Keycloak or Google
Workspace, over OpenID Connect (OIDC) or SAML 2.0. One IdP serves the whole
instance. It is a paid feature: it needs the commercial
image and a license key, see
[Getting a license key](../getting-started/licensing.md#getting-a-license-key).

Passwords keep working next to SSO unless you [enforce it](#enforcing-sso).
When a sign-in fails, see [SSO troubleshooting](./sso-troubleshooting.md).

## Before you start

Set two variables and restart the tracker, see
[Configuration](../getting-started/configuration.md):

- `PUBLIC_BASE_URL`: the https address people open, without a path, for example
  `https://tracker.example.com`. Plain `http` works only for `localhost`.
- `SSO_ENCRYPTION_KEY`: 32 random bytes in base64, for example from
  `openssl rand -base64 32`. It encrypts the client secret and the key of the
  SAML certificate. Keep it with your backups: when it is lost or changed, SSO
  stops. Enter the client secret again (OIDC), or generate a new SP certificate
  in the settings and upload the new SP metadata to the IdP (SAML).

## The settings page

Everything lives in **Admin → Settings → Sign-in and SSO**. Each block saves on
its own, with its own button. The ⓘ icons explain a setting in more detail.

| Block | What it is for | Needed |
| --- | --- | --- |
| Status at the top | Shows whether single sign-on is on, what is still missing, and turns it on or off. | Yes |
| **Identity provider** | **Copy to your identity provider** holds the addresses you give the IdP. **From your identity provider** holds what the IdP gives you. | Yes |
| **Who can sign in** | Which e-mail domains may sign in, whether new accounts are created, and the admin group. | Yes |
| **Teams from groups** | Puts people into teams, and so into projects, by their IdP groups. | Recommended |
| **Security** | **Require single sign-on** for everyone except break-glass accounts, and **SCIM provisioning**. | No |

## Set it up

### 1. Register rurdesk at your IdP

Create an application at the IdP and give it the values from **Copy to your
identity provider** in the **Identity provider** block. Pick the protocol in the
block header first:

- **OIDC:** a web application with the **Redirect URI**
  (`<PUBLIC_BASE_URL>/api/public/sso/oidc/callback`). Note the client ID and the
  client secret the IdP gives you.
- **SAML:** an application with the **ACS URL** and the **SP entity ID**, or let
  the IdP read the **SP metadata URL**. rurdesk signs its requests and can read
  encrypted assertions.

Make sure the IdP sends the e-mail address, and the groups if you want
[group mapping](#groups-and-teams). [Identity providers](#identity-providers)
lists the settings for the common ones.

### 2. Fill in the connection and test it

1. Under **From your identity provider**, fill in:
   - **OIDC:** the **Issuer URL**, for example `https://login.example.com/realms/acme`,
     the **Client ID** and the **Client secret**.
   - **SAML:** the **Metadata URL**, or paste the metadata XML, not both. Open
     **Attribute names** only when the IdP does not use `email`, `name` and
     `groups`. With OIDC the same place is called **Claim names**.
2. Pick **Save and test**. The browser goes to the IdP, you sign in there and
   come back to the settings page.

Below the fields you now see what the IdP sent (e-mail, name, groups) and what a
real sign-in of that person would do. The test never creates an account or a
session. Warnings below the result point to settings that need a fix, see
[Troubleshooting](./sso-troubleshooting.md#warnings-after-a-test).

A first test often says the sign-in would **be rejected**: the rules from the
next step are still empty.

### 3. Choose who can sign in

1. Under **Allowed e-mail domains**, write the domains of your company, one per
   line, for example `example.com`. Nobody new can sign in until at least one
   domain is listed.
2. Turn on **Create accounts on first sign-in** if anyone from these domains may
   get an account. Leave it off to let in only people who already have an
   account with the same e-mail.
3. Optionally fill in **Admin group** with the name of an IdP group: its
   members become admins.
4. Pick **Save**.

Rules apply right away, also to an active connection. Run **Save and test**
again: the result should now say **create a new account** or **link the
existing account with this e-mail**.

### 4. Map groups to teams

A person who is in no project sees the page for creating a first project after
the first sign-in, and nothing else. To bring people straight into their
projects, map their IdP groups to teams that are members of those projects:

1. Make sure the teams exist and are added to the projects
   (see [User management](./user-management.md)).
2. Under **Teams from groups**, fill in **Group** exactly as the IdP sends it,
   or click a group from **Groups from the last test**, and pick the **Team**.
3. Pick **Add**.

The mapping takes effect on the next sign-in of each person. More in
[Groups and teams](#groups-and-teams).

### 5. Turn it on

The status at the top lists what is done: **Connection tested**, **Allowed
e-mail domains set** and, optionally, **Groups mapped to teams**. Pick **Turn
on** there. It stays disabled until the current connection has passed a test.
The status then says **Single sign-on is on**.

From then on the login page shows only the company sign-in. People with a
password, such as an administrator, pick **Sign in with email and password**
below it, and **Back to company sign-in** returns.

### 6. Try it as a regular person

Open the tracker in a private browser window, so your admin session stays, and
pick **Sign in with your company account**. Sign in at the IdP as a regular
person. You land in the tracker; **Admin → Users** lists the person with the
**SSO** label. If the login page shows an error instead, look it up in
[Troubleshooting](./sso-troubleshooting.md).

### Changing the connection later

While single sign-on is on, a saved change of the connection does not touch the
live sign-in. Below the fields the page says that sign-in still uses the
previous connection. Pick **Save and test**, then **Apply**. SAML metadata from
a URL is downloaded on every test and refreshed once a day while SSO is on; when
the download fails, the last copy stays in use and the next try comes 15 minutes
later.

Sign-out ends the rurdesk session only, not the session at the IdP.

## How accounts are matched

On every SSO sign-in the tracker looks for an account in this order:

1. An account already linked to this IdP identity signs in, even if its domain
   was removed from the list. Disable the account to stop it.
2. The e-mail must be present, in an allowed domain and not marked as
   unverified by the IdP.
3. An account with the same e-mail (any letter case) gets linked. Agent
   accounts and accounts already linked to another identity from the same IdP
   are never linked.
4. With no account, a new one without a password is created if automatic
   creation is on. Otherwise the person is asked to contact an administrator.

The name, and the e-mail if it stays in an allowed domain, follow the IdP on
every sign-in.

SSO sessions last 24 hours. **Stay signed in** extends them to 30 days only
while [SCIM](#scim-provisioning) is on with a token, because only then can the
IdP end the session of someone who leaves. The login page shows it accordingly:
with SCIM the checkbox sits under **Sign in with your company account** too,
without SCIM it appears only in the password form.

Signing out of rurdesk does not sign the person out of the IdP. While the IdP
session lasts, **Sign in with your company account** lets them in without a
password; how long that is depends on the session settings of the IdP.

## Groups and teams

Under **Teams from groups** you map a group value from the IdP to a rurdesk team.
The value must match exactly what the IdP sends: Okta sends the group name,
Microsoft Entra ID the group object ID, Keycloak the group path such as
`/developers` (or the plain name when **Full group path** is off in its group
mapper). After a test, the groups it saw can be added with one click.

On every SSO sign-in the IdP manages the membership of mapped teams: the person
joins the teams of their groups and leaves the mapped teams of groups they are no
longer in. Teams without a mapping are never touched, and nobody gets a
notification about these changes. The **SSO** label in **Admin → Users** marks
mapped teams; a manual change of an SSO user in such a team lasts only until
their next sign-in.

The groups come from the claim named in **Groups claim** under **Claim names**
(`groups` by default, `roles` for Entra app roles). Okta needs **Request the
groups scope**. An empty
list removes the person from every mapped team.

IdPs leave the claim out for a person who is in no group. rurdesk remembers
whether the IdP has ever sent groups for each person:

- If it has, a missing claim means no groups: the person leaves every mapped
  team and loses the role from the admin group.
- If it never has, the IdP is probably not set up to send groups, so the sync is
  skipped and memberships stay as they are.

Entra leaves the claim out for people in more than 200 groups. Then the sync is
always skipped, so use app roles or group filtering there.

**Admin group** under **Who can sign in** makes the members of one group admins and takes
the role away when they leave it. Leave it empty to manage admins in rurdesk. The
last active admin with a password never loses the role this way.

## Fixing a wrong link

**Admin → Users → Edit** shows the SSO identity of an account, marked with the
**SSO** label in the list:

- **Unlink** removes the identity. The next SSO sign-in links an account by
  e-mail again.
- **Move** puts the identity on another account and disables this one. Use it
  when the IdP was linked to a duplicate account. The data of the two accounts is
  not merged. The target cannot be an agent, a disabled account or an account that
  already has an identity from the same IdP.

## Enforcing SSO

**Require single sign-on** under **Security** makes SSO the only way in for
people. While it is on,
only break-glass accounts sign in with a password. Everyone else gets the same
error as with a wrong password, so the login page does not reveal whether a
password is right. The login page offers the password form under **Emergency
sign-in with a password**, with a note that only break-glass accounts can use
it. Agents and API keys, personal ones included, keep working.

Before you enforce SSO, mark at least one admin as a break-glass account in
**Admin → Users → Edit**. Only an active admin with a password and no SSO
identity qualifies. Use a technical account whose password sits in a vault, not
a personal one, and check its **Last sign-in** in the list of break-glass
accounts now and then.

- Requiring SSO signs out everyone except the break-glass accounts. Your own session
  stays. Everyone else signs in once through SSO.
- While SSO is required, the last break-glass account cannot be disabled,
  deleted, demoted or lose the flag.
- **Stop requiring** or **Turn off** in the status at the top lets passwords
  work again.
- Accounts linked to SSO keep their old passwords. They work again as soon as
  the requirement stops, also when the license expires. To cut off people who left
  the company, disable their accounts.

## SCIM provisioning

With SCIM 2.0 the IdP creates, updates and disables accounts on its own. Only
users are supported, not groups: team membership still comes from the
[group mapping](#groups-and-teams) on sign-in.

1. Under **Security**, turn on **SCIM provisioning** and pick **Generate
   token**. Copy the token right away, it is shown only once.
2. In the IdP, set the **SCIM base URL** from **Copy to your identity
   provider** (`<PUBLIC_BASE_URL>/api/scim/v2`) and the token as a bearer token.
3. List the e-mail domains under **Who can sign in**. SCIM refuses addresses
   outside them.

What SCIM does:

- **Create** makes an account without a password. If an account with the same
  e-mail exists, SCIM takes it over instead of creating a second one. The first
  SSO sign-in then links it.
- **Deactivate** (`active` set to false) and **delete** disable the account at
  once: its sessions end and its API keys stop working, even when the person is
  the only owner of a project. The account and its data stay. A deleted account
  disappears from SCIM until the IdP creates it again.
- **Reactivate** enables the account again, also when an administrator disabled
  it by hand. The IdP is the source of truth for accounts it manages; the
  **SCIM** label marks them in **Admin → Users**.
- The last active admin is never deactivated; SCIM gets an error instead.
- Agents and break-glass accounts are invisible to SCIM.

The account e-mail is the primary address from `emails`, otherwise the work
address, otherwise `userName`.

The account name is `displayName`, otherwise `name.formatted`, otherwise the
given and the family name together. rurdesk stores the name as a whole, so an
update that carries only one of the two parts leaves the name as it is.

- **Microsoft Entra ID:** create an enterprise application with provisioning,
  use the tenant URL and the token from rurdesk. Map `userName` to `mail`, not to
  the default `userPrincipalName`, so the e-mail matches the one in the token.
- **Okta:** add SCIM provisioning to the app integration, use **HTTP Header**
  authentication with the token, and turn on creating, updating and deactivating
  users.

**New token** replaces the old one at once. **Revoke token** stops
provisioning until you generate a new one.

## SAML details

- The IdP must sign the response or the assertion. Unsigned, unsolicited
  (IdP-initiated) and replayed responses are refused.
- The account is identified by the NameID. Use a persistent NameID, or set a
  **Subject attribute** with a stable ID (for example the Entra object ID). A
  transient NameID is refused. With an e-mail or unspecified NameID the test
  warns you: when the address changes at the IdP, the next sign-in links the
  account anew.
- If the e-mail attribute is missing, an e-mail NameID is used as the e-mail.

## Identity providers

- **Microsoft Entra ID:** use the issuer of your tenant,
  `https://login.microsoftonline.com/<tenant-id>/v2.0`. The `common`,
  `organizations` and `consumers` issuers are refused. Entra often sends no
  `email`; set the e-mail claim to `preferred_username` or add the optional
  `email` claim.
- **Okta:** use `https://<your-org>.okta.com` (or your authorization server) as
  the issuer.
- **Keycloak:** use `https://<host>/realms/<realm>` as the issuer.
- **Google Workspace:** use `https://accounts.google.com`. Google must send the
  `hd` claim with an allowed domain, so personal Google accounts cannot sign in.
  Google sends no groups over OIDC; for group mapping set up a SAML app in the
  Google Admin console and add a groups attribute.
- **Microsoft Entra ID over SAML:** create an enterprise application with SAML,
  enter the ACS URL and the SP entity ID, and use the **App Federation Metadata
  URL** as the metadata URL. Entra sends the e-mail as
  `http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress` and the
  groups as `http://schemas.microsoft.com/ws/2008/06/identity/claims/groups`;
  put these names into the attribute fields.
- **Keycloak over SAML:** create a SAML client with the SP entity ID as client
  ID and use `https://<host>/realms/<realm>/protocol/saml/descriptor` as the
  metadata URL. Add mappers for the e-mail, name and groups attributes.

Other multi-tenant providers are not supported.

## When the license expires

SSO sign-in stops and the login page says so. Passwords keep working, also
while SSO is enforced: enforcement pauses and comes back after the renewal.
Accounts created by SSO have no password: an administrator can set one in
**Admin → Users**. The configuration stays saved and works again after renewal.
SCIM keeps deactivating and deleting accounts without a license, so people who
leave still lose access; creating and updating accounts waits for the renewal.
An administrator can still turn SSO off, stop enforcing it and turn SCIM off
without a license.

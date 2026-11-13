---
title: SSO troubleshooting
description: What each single sign-on error on the login page and in the SSO settings means, and how to fix it.
stackTables: true
---

# SSO troubleshooting

This page lists every message single sign-on (SSO) can show, what causes it and
who fixes it. For the setup itself see [Single sign-on](./sso.md).

## If you cannot sign in

Most SSO errors can only be fixed by an administrator of the tracker or of
your identity provider (IdP). Send them the exact message from the login page
and the time you tried.

- **Incorrect email or password** while the login page offers **Sign in with
  your company account**: your company may have turned password sign-in off. Use
  the company sign-in.
- **The sign-in took too long or was already used**: start again from the login
  page. Do not use the back button during the sign-in.
- **Your identity provider did not complete the sign-in**: you may have
  cancelled the sign-in, or your company account has no access to rurdesk at the
  IdP. Try again, then ask your administrator.
- After the first sign-in you see only the page for creating a first project:
  you are not in any project yet. Ask your administrator to add you to a team or
  a project.

## Finding the cause

Every failed SSO sign-in leaves a warning in the tracker log with the message
`sso: sign-in failed`. The `ssoError` field holds the code from the tables
below and the `error` field the details, for example the answer of the IdP:

```bash
docker compose logs rurdesk | grep "sso: sign-in failed"
```

The fastest way to see what the IdP sends is **Save and test** in **Admin →
Settings → Sign-in and SSO**. It runs the whole sign-in for you, shows the
e-mail, name and groups from the IdP and what a real sign-in would do, without
creating an account or a session.

## Messages on the login page

| Message | Code | Cause | Fix |
| --- | --- | --- | --- |
| Your e-mail domain cannot sign in here. | `domain_not_allowed` | The domain of the e-mail is not under **Allowed e-mail domains**, or the list is empty. With Google, the `hd` claim is missing or not allowed. | Add the domain under **Who can sign in** and pick **Save**. |
| You have no account here yet. Ask your administrator for access. | `account_not_provisioned` | No account has this e-mail and automatic creation is off. | Turn on **Create accounts on first sign-in** under **Who can sign in**, or create the account in **Admin → Users** with the same e-mail, or provision it over SCIM. |
| Your account is disabled. | `account_disabled` | The account linked to this person, or the account with this e-mail, is disabled. | Enable it in **Admin → Users** if the person should have access. |
| Your account cannot use company sign-in. Ask your administrator. | `account_not_linkable` | The account with this e-mail is an agent or a break-glass account, or it is already linked to another identity from the same IdP, for example after the person was recreated at the IdP. | For the last case, open **Admin → Users → Edit** and pick **Unlink**; the next sign-in links the account again. Break-glass accounts never use SSO. |
| More accounts use your e-mail address. Ask your administrator. | `email_ambiguous` | Two or more accounts have this e-mail in a different letter case. | Change the e-mail of the accounts that do not belong to the person. |
| Your identity provider did not send an e-mail address. | `email_missing` | The IdP sends no e-mail in the claim or attribute set in the connection. | Fix the **E-mail claim** or **E-mail attribute**, or make the IdP send it. With Microsoft Entra ID use `preferred_username` or add the optional `email` claim. |
| Your identity provider says your e-mail address is not verified. | `email_not_verified` | The IdP marks the e-mail as unverified. | Verify the address at the IdP. |
| Your identity provider sent no lasting identifier. Ask your administrator. | `saml_subject_unsupported` | SAML only: the NameID is transient and no **Subject attribute** is set. | Set a persistent NameID at the IdP, or fill in **Subject attribute** with a stable ID. |
| Your identity provider did not complete the sign-in. Try again. | `idp_error` | The IdP returned an error, or its answer could not be verified: a wrong client secret, a changed signing certificate, a different issuer or audience, or a clock off by minutes. | Check the details in the log, then run **Save and test**. |
| The sign-in took too long or was already used. Try again. | `invalid_state` | More than 10 minutes passed at the IdP, the page was reloaded, or the same sign-in finished twice. | Start again from the login page. |
| Company sign-in is not available right now. | `sso_unavailable` | SSO was turned off, the license does not include SSO, `PUBLIC_BASE_URL` is not valid, or the IdP could not be reached (OIDC discovery, SAML metadata). | Check the license, `PUBLIC_BASE_URL` and that the tracker can reach the IdP. |
| Company sign-in failed. Try again. | `internal` | An error on the tracker side, for example the database or Redis. | Look for the details in the log. |
| The sign-in link is invalid or has expired. | | The last step took more than a minute, or the sign-in started in another browser or under another address than `PUBLIC_BASE_URL`. | Start again from the login page, in one browser, at the address from `PUBLIC_BASE_URL`. |
| This account cannot sign in. | | The account was disabled during the sign-in. | Enable it in **Admin → Users**. |

The login page can also show these notes:

- **Company sign-in is temporarily unavailable. If you have no password, ask
  your administrator.** SSO is turned on, but the license does not include it
  or `PUBLIC_BASE_URL` is not valid. Passwords keep working. See
  [When the license expires](./sso.md#when-the-license-expires).
- **No Sign in with your company account button.** SSO is not active. Open the
  settings and check that the status at the top says **Single sign-on is on**.

## Errors in the test

**Save and test** shows an error below the connection fields instead of the
result. The line below it holds the details from the IdP or from the
verification.

| Message | Cause | Fix |
| --- | --- | --- |
| The identity provider returned an error. | The IdP refused the sign-in, for example an unknown client ID, a redirect URI the IdP does not know, or a user without access to the application. | Compare the **Redirect URI** or **ACS URL** and the client ID with the application at the IdP. |
| The token from the identity provider could not be verified. | A wrong client secret, a different issuer, an unsigned or wrongly signed SAML response, or an audience that is not the **SP entity ID**. | Check the client secret and the issuer. For SAML, download the metadata again and check the **SP entity ID** at the IdP. |
| The token has no subject. | The IdP sent no user ID. | Check the claims or the NameID at the IdP. |
| The assertion has a transient NameID. Configure a persistent NameID or a subject attribute. | SAML with a transient NameID. | Set a persistent NameID or a **Subject attribute**. |
| The draft changed during the test. Test it again. | Someone saved the connection while you were at the IdP. | Run **Save and test** again. |
| Single sign-on is unavailable. Check the license and PUBLIC_BASE_URL. | The license does not include SSO or `PUBLIC_BASE_URL` is not valid. | Fix the license or the variable and restart the tracker. |
| The test failed on the server. | An error on the tracker side. | Look for the details in the log. |

## Warnings after a test

A test can pass and still show warnings. Each one means a real sign-in of this
person will not work as you expect.

| Warning | What to do |
| --- | --- |
| The identity provider sent no e-mail address. Check the e-mail claim. | Fix the **E-mail claim** or **E-mail attribute**. |
| The e-mail domain is not in the allowed domains. | Add the domain under **Who can sign in** and pick **Save**. |
| Google sent no hd claim. Only Google Workspace accounts can sign in. | Sign in with a Google Workspace account, not a personal one. |
| The identity provider says the e-mail address is not verified. | Verify the address at the IdP. |
| The token has no groups claim. If this person is in no group, that is expected. If the identity provider never sends groups, group mapping is skipped. | Test with a person who is in a group. If the claim is still missing, make the IdP send groups and check the **Groups claim** or **Groups attribute**. Okta needs **Request the groups scope**. |
| The user is in too many groups for the token (Entra limit 200). Group mapping will be skipped. | Use app roles with the groups claim `roles`, or limit the groups the application sends. |
| The NameID is an e-mail address or unspecified. | Prefer a persistent NameID, or set a **Subject attribute**. |

## Errors when saving the settings

| Message | Fix |
| --- | --- |
| SSO_ENCRYPTION_KEY is not set. | Set `SSO_ENCRYPTION_KEY` to 32 random bytes in base64 and restart the tracker. |
| PUBLIC_BASE_URL must be an https address without a path. | Set `PUBLIC_BASE_URL`, for example `https://tracker.example.com`, and restart. Plain `http` works only for `localhost`. |
| The issuer must be an https URL. | Use the `https` issuer of the IdP. |
| Multi-tenant Microsoft issuers are not supported. Use the issuer of your tenant. | Use `https://login.microsoftonline.com/<tenant-id>/v2.0`. |
| The identity provider could not be reached. Check the issuer URL. | Check the issuer for typos and that the tracker can reach it over the network. |
| Fill in the issuer URL and the client ID. | Fill in both fields. |
| Enter the client secret. | The first OIDC connection needs the client secret. |
| The saved client secret cannot be decrypted. Enter it again. | `SSO_ENCRYPTION_KEY` changed. Enter the client secret again and save. |
| Fill in the metadata URL or paste the metadata XML of your identity provider. | SAML needs one of the two. |
| The metadata URL must be an https address. | Use the `https` metadata URL of the IdP. |
| The metadata of your identity provider could not be downloaded. | Open the URL from the tracker host and check that it returns XML. |
| The metadata is not valid SAML metadata of an identity provider. | Use the IdP metadata, not the metadata of rurdesk. |
| Test the current draft before you activate it. | Run **Save and test**, then **Turn on**, or **Apply** while single sign-on is on. |
| One of the domains is not valid. | Write only domains, one per line, such as `example.com`, without `@`. |

## Other situations

- **Locked out while SSO is enforced.** Sign in with a break-glass account
  under **Emergency sign-in with a password** and pick **Stop requiring** under **Security**. When
  no break-glass account works, stop enforcing in the database; passwords work
  again right away:

  ```bash
  docker compose exec rurdesk.db psql -U rurdesk -d rurdesk \
    -c "UPDATE users.sso_settings SET is_enforced = false"
  ```
- **`SSO_ENCRYPTION_KEY` was lost or changed.** The settings page says so. Enter
  the client secret again (OIDC), or pick **Generate a new SP certificate** and
  upload the new SP metadata to the IdP (SAML).
- **A person is linked to the wrong account.** Use **Unlink** or **Move** in
  **Admin → Users → Edit**, see [Fixing a wrong link](./sso.md#fixing-a-wrong-link).
- **Teams do not change after a change at the IdP.** Group mapping runs on
  sign-in. The person has to sign out and sign in again. If the IdP has never
  sent groups for this person, the sync is skipped, see
  [Groups and teams](./sso.md#groups-and-teams).
- **Company sign-in lets people in without a password after they signed out.**
  Signing out of rurdesk does not end the session at the IdP. Sign out at the
  IdP too, or use a private browser window to sign in as someone else.

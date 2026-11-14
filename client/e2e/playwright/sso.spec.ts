import { expect, test } from '@playwright/test';
import { authHeaders } from './support/auth';
import { BOOTSTRAP_USER } from './support/bootstrap-user';
import { Interaction } from './support/interaction';
import { IdpIdentity, MOCK_IDP_ISSUER, Sso } from './support/sso';
import { createUser } from './support/user';

test.describe.configure({ mode: 'serial' });

test.describe('single sign-on', () => {
    const label = `sso-${Date.now()}`;
    const backend = `SSO Backend ${label}`;
    const support = `SSO Support ${label}`;
    const manual = `SSO Manual ${label}`;
    const person: IdpIdentity = {
        subject: `sub-${label}`,
        email: `${label}@example.com`,
        name: 'Jana SSO',
        groups: ['dev']
    };
    let adminToken = '';
    let idManualTeam = 0;
    const idTeams = new Map<string, number>();

    test.beforeAll(async ({ request, baseURL }) => {
        adminToken = await Sso.adminToken(request, baseURL!);
        for (const [project, team] of [
            [backend, backend],
            [support, support]
        ]) {
            const idTeam = await Sso.createTeam(request, baseURL!, adminToken, team);
            idTeams.set(team, idTeam);
            const idProject = await Sso.createProject(request, baseURL!, adminToken, project);
            await Sso.shareProjectWithTeam(request, baseURL!, adminToken, idProject, idTeam);
        }
        idManualTeam = await Sso.createTeam(request, baseURL!, adminToken, manual);
    });

    // Scenario:
    // - the admin fills in the OIDC connection and runs the connection test at the IdP
    // - the test result offers the group "dev", which the admin maps to the Backend team
    // - the admin allows the example.com domain with automatic accounts and activates SSO
    // - a new person signs in through the IdP, gets an account and sees the Backend project
    test('turns SSO on and signs in a new person', async ({ page, browser, request, baseURL }) => {
        await Interaction.login(page, { ...BOOTSTRAP_USER });
        await page.goto('/admin/settings/sso');
        await page.getByTestId('sso-issuer').fill(MOCK_IDP_ISSUER);
        await page.getByTestId('sso-client-id').fill('rurdesk');
        await page.getByTestId('sso-client-secret').fill('mock-secret');
        await page.getByTestId('sso-save-and-test').click();
        await Sso.signInAtIdp(page, { ...person, subject: 'admin-test' });

        await page.waitForURL(/\/admin\/settings\/sso/);
        await expect(page.getByTestId('sso-test-success')).toBeVisible();
        await page.getByTestId('sso-domains').fill('example.com');
        await page.getByTestId('sso-auto-create').check();
        await page.getByTestId('sso-save-rules').click();
        await page.getByTestId('sso-mapping-suggestion').filter({ hasText: 'dev' }).click();
        await Interaction.pickOption(page, '#sso-mapping-team', backend);
        await page.getByTestId('sso-mapping-add').click();
        await expect(page.getByTestId('sso-mapping-row')).toContainText(backend);
        await page.getByTestId('sso-activate').click();
        await expect(page.getByTestId('sso-status')).toContainText('is on');

        const context = await browser.newContext();
        const personPage = await context.newPage();
        await Sso.signIn(personPage, person);
        await expect(personPage.getByTestId('avatar-trigger')).toBeVisible();
        const projects = await Sso.projectNames(
            request,
            baseURL!,
            await Sso.sessionToken(personPage)
        );
        expect(projects).toContain(backend);
        expect(projects).not.toContain(support);
        await context.close();
    });

    // Scenario:
    // - the admin maps the IdP group "support" to the Support team
    // - the admin adds the person by hand to a team that has no mapping
    // - the person signs in again, now only in the IdP group "support"
    // - the mapped Backend team is gone, the Support project is visible
    // - the hand-made membership of the unmapped team stays
    test('follows the groups of the IdP on the next sign-in', async ({
        browser,
        request,
        baseURL
    }) => {
        const mapped = await request.post(`${baseURL}/api/private/admin/sso/mapping`, {
            headers: authHeaders(adminToken),
            data: { groupValue: 'support', idTeam: idTeams.get(support) }
        });
        expect(mapped.ok()).toBe(true);
        const idUser = await Sso.idOfUser(request, baseURL!, adminToken, person.email);
        const added = await request.post(`${baseURL}/api/private/admin/team/member`, {
            headers: authHeaders(adminToken),
            data: { idTeam: idManualTeam, idUser }
        });
        expect(added.ok()).toBe(true);

        const context = await browser.newContext();
        const page = await context.newPage();
        await Sso.signIn(page, { ...person, groups: ['support'] });
        const projects = await Sso.projectNames(request, baseURL!, await Sso.sessionToken(page));
        await context.close();

        expect(projects).toContain(support);
        expect(projects).not.toContain(backend);
        const members = await request.get(
            `${baseURL}/api/private/admin/team/${idManualTeam}/member`,
            { headers: authHeaders(adminToken) }
        );
        expect(await members.text()).toContain(person.email);
    });

    // Scenario:
    // - a person with a local account and an own project signs in through SSO with the same e-mail
    // - the account is linked, not duplicated, so the person sees the own project
    // - the admin marks a break-glass account and enforces SSO in the settings
    // - a regular password sign-in fails like a wrong password, the break-glass account signs in
    test('links an existing account, enforces SSO and keeps the break-glass way in', async ({
        page,
        browser,
        request,
        baseURL
    }) => {
        const local = await createUser(request, baseURL!, `${label}-local`);
        const localLogin = await request.post(`${baseURL}/api/public/login`, {
            data: { email: local.email, password: local.password }
        });
        const localToken = ((await localLogin.json()) as { token: string }).token;
        const ownProject = `SSO Own ${label}`;
        await Sso.createProject(request, baseURL!, localToken, ownProject);

        const context = await browser.newContext();
        const linkedPage = await context.newPage();
        await Sso.signIn(linkedPage, {
            subject: `sub-local-${label}`,
            email: local.email,
            name: local.name,
            groups: []
        });
        expect(
            await Sso.projectNames(request, baseURL!, await Sso.sessionToken(linkedPage))
        ).toContain(ownProject);
        await context.close();

        const breakGlass = await createUser(request, baseURL!, `${label}-vault`);
        const idBreakGlass = await Sso.idOfUser(request, baseURL!, adminToken, breakGlass.email);
        for (const body of [{ isAdmin: true }, { isBreakGlass: true }]) {
            const res = await request.patch(`${baseURL}/api/private/admin/user/${idBreakGlass}`, {
                headers: authHeaders(adminToken),
                data: body
            });
            expect(res.ok()).toBe(true);
        }

        await Interaction.login(page, { ...BOOTSTRAP_USER });
        await page.goto('/admin/settings/sso');
        await expect(page.getByTestId('sso-break-glass-row')).toContainText(breakGlass.email);
        await page.getByTestId('sso-enforce-enable').click();
        await Sso.confirm(page);
        await expect(page.getByTestId('sso-enforce-on')).toBeVisible();

        const regular = await request.post(`${baseURL}/api/public/login`, {
            data: { email: local.email, password: local.password }
        });
        const wrong = await request.post(`${baseURL}/api/public/login`, {
            data: { email: local.email, password: `${local.password}-nope` }
        });
        expect(regular.status()).toBe(wrong.status());
        expect(await regular.text()).toBe(await wrong.text());

        const vaultContext = await browser.newContext();
        const vaultPage = await vaultContext.newPage();
        await vaultPage.goto('/login');
        await vaultPage.getByTestId('login-use-password').click();
        await vaultPage.locator('input[formcontrolname="email"]').fill(breakGlass.email);
        await vaultPage.locator('input[formcontrolname="password"]').fill(breakGlass.password);
        await vaultPage.getByRole('button', { name: /login/i }).click();
        await expect(vaultPage.getByTestId('avatar-trigger')).toBeVisible();
        await vaultContext.close();

        const off = await request.put(`${baseURL}/api/private/admin/sso/enforce`, {
            headers: authHeaders(await Sso.sessionToken(page)),
            data: { isEnforced: false }
        });
        expect(off.ok()).toBe(true);
    });

    // Scenario:
    // - the admin turns SCIM on and generates a token
    // - the person is signed in through SSO
    // - the IdP deactivates the person over SCIM
    // - the next action of the person ends on the login page and a new SSO sign-in is refused
    test('ends the session of a person deactivated over SCIM', async ({
        browser,
        request,
        baseURL
    }) => {
        adminToken = await Sso.adminToken(request, baseURL!);
        const enabled = await request.put(`${baseURL}/api/private/admin/sso/scim`, {
            headers: authHeaders(adminToken),
            data: { isEnabled: true }
        });
        expect(enabled.ok()).toBe(true);
        const generated = await request.post(`${baseURL}/api/private/admin/sso/scim/token`, {
            headers: authHeaders(adminToken)
        });
        const { token: scimToken } = (await generated.json()) as { token: string };

        const context = await browser.newContext();
        const page = await context.newPage();
        await Sso.signIn(page, person);
        await expect(page.getByTestId('avatar-trigger')).toBeVisible();

        const idUser = await Sso.idOfUser(request, baseURL!, adminToken, person.email);
        const deactivated = await request.patch(`${baseURL}/api/scim/v2/Users/${idUser}`, {
            headers: { ...authHeaders(scimToken), 'Content-Type': 'application/scim+json' },
            data: {
                schemas: ['urn:ietf:params:scim:api:messages:2.0:PatchOp'],
                Operations: [{ op: 'Replace', path: 'active', value: 'False' }]
            }
        });
        expect(deactivated.status()).toBe(200);

        await page.reload();
        await page.waitForURL(/\/login/);
        await page.getByTestId('login-sso').click();
        await Sso.signInAtIdp(page, person);
        await expect(page.getByTestId('login-sso-error')).toBeVisible();
        await context.close();
    });
});

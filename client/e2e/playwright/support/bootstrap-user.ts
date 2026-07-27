import { APIRequestContext, expect } from '@playwright/test';

// Fixed credentials for the instance bootstrap (first) user. Public registration
// is a one-time bootstrap — the very first user becomes the instance admin and
// the endpoint then closes (see api user_controller.Register).
export const BOOTSTRAP_USER = {
    name: 'E2E Admin',
    email: 'e2e-admin@example.com',
    password: 'Passw0rd!23'
};

/**
 * Ensures the bootstrap user exists on the target stack. Idempotent: 200 on a
 * fresh DB (we just created the first user), 403 if it already bootstrapped the
 * stack on an earlier run. Anything else is a real failure worth surfacing.
 *
 * A 403 alone does NOT prove the user is ours — it only means registration is
 * closed, which is equally true when somebody else bootstrapped the instance
 * (e.g. the suite was pointed at the shared dev stack). We therefore verify we
 * can actually log in, so that case fails here with its cause rather than three
 * steps later as a baffling "element not found".
 */
export async function ensureBootstrapUser(
    request: APIRequestContext,
    baseURL: string
): Promise<void> {
    const register = await request.post(`${baseURL}/api/public/register`, {
        data: BOOTSTRAP_USER
    });
    expect([200, 403], `unexpected /register status ${register.status()}`).toContain(
        register.status()
    );

    const login = await request.post(`${baseURL}/api/public/login`, {
        data: { email: BOOTSTRAP_USER.email, password: BOOTSTRAP_USER.password }
    });
    expect(
        login.status(),
        `cannot log in as the bootstrap user ${BOOTSTRAP_USER.email} on ${baseURL} ` +
            `(register returned ${register.status()}, login returned ${login.status()}). ` +
            `This instance was most likely bootstrapped by a different user — the e2e ` +
            `suite needs a FRESH stack, not the shared dev one. Run "npm run e2e:stack".`
    ).toBe(200);
}

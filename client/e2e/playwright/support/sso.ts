import { APIRequestContext, Page, expect } from '@playwright/test';
import { authHeaders } from './auth';
import { BOOTSTRAP_USER } from './bootstrap-user';

export const MOCK_IDP_ISSUER = 'http://localhost:18080/default';

export interface IdpIdentity {
    subject: string;
    email: string;
    name: string;
    groups: string[];
}

export abstract class Sso {
    public static async adminToken(request: APIRequestContext, baseURL: string): Promise<string> {
        const login = await request.post(`${baseURL}/api/public/login`, {
            data: { email: BOOTSTRAP_USER.email, password: BOOTSTRAP_USER.password }
        });
        expect(login.status()).toBe(200);
        return ((await login.json()) as { token: string }).token;
    }

    public static async signInAtIdp(page: Page, identity: IdpIdentity): Promise<void> {
        await page.waitForURL(/localhost:18080\/default\/authorize/);
        await page.locator('input[name="username"]').fill(identity.subject);
        await page.locator('textarea[name="claims"]').fill(
            JSON.stringify({
                email: identity.email,
                email_verified: true,
                name: identity.name,
                groups: identity.groups
            })
        );
        await page.locator('input[type="submit"]').click();
    }

    public static async signIn(page: Page, identity: IdpIdentity): Promise<void> {
        await page.goto('/login');
        await page.getByTestId('login-sso').click();
        await Sso.signInAtIdp(page, identity);
        await page.waitForURL(url => !/\/(login|auth\/sso)/.test(url.pathname));
    }

    public static async sessionToken(page: Page): Promise<string> {
        const token = await page.evaluate(() => localStorage.getItem('Authorization'));
        expect(token).toBeTruthy();
        return token!;
    }

    public static async projectNames(
        request: APIRequestContext,
        baseURL: string,
        token: string
    ): Promise<string[]> {
        const res = await request.get(`${baseURL}/api/private/project`, {
            headers: authHeaders(token)
        });
        expect(res.status()).toBe(200);
        return ((await res.json()) as { name: string }[]).map(project => project.name);
    }

    public static async createTeam(
        request: APIRequestContext,
        baseURL: string,
        token: string,
        name: string
    ): Promise<number> {
        const res = await request.post(`${baseURL}/api/private/admin/team`, {
            headers: authHeaders(token),
            data: { name, color: '#3366ff' }
        });
        expect(res.status()).toBe(200);
        return ((await res.json()) as { idTeam: number }).idTeam;
    }

    public static async createProject(
        request: APIRequestContext,
        baseURL: string,
        token: string,
        name: string
    ): Promise<number> {
        const res = await request.post(`${baseURL}/api/private/project`, {
            headers: authHeaders(token),
            data: { name, color: '#123456' }
        });
        expect(res.status()).toBe(200);
        return ((await res.json()) as { idProject: number }).idProject;
    }

    public static async shareProjectWithTeam(
        request: APIRequestContext,
        baseURL: string,
        token: string,
        idProject: number,
        idTeam: number
    ): Promise<void> {
        const res = await request.post(`${baseURL}/api/private/project/${idProject}/member/team`, {
            headers: authHeaders(token),
            data: { idTeam, role: 'member' }
        });
        expect(res.ok()).toBe(true);
    }

    public static async idOfUser(
        request: APIRequestContext,
        baseURL: string,
        token: string,
        email: string
    ): Promise<number> {
        const res = await request.get(`${baseURL}/api/private/admin/user`, {
            headers: authHeaders(token)
        });
        const users = (await res.json()) as { idUser: number; email: string }[];
        const user = users.find(candidate => candidate.email === email);
        expect(user, `user ${email}`).toBeTruthy();
        return user!.idUser;
    }

    public static async confirm(page: Page): Promise<void> {
        await page.locator('.ui-confirm-panel button').last().click();
    }
}

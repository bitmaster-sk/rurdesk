import { APIRequestContext, expect } from '@playwright/test';
import { BOOTSTRAP_USER } from './bootstrap-user';

export interface TestUser {
    name: string;
    email: string;
    password: string;
}

export async function createUser(
    request: APIRequestContext,
    baseURL: string,
    label: string
): Promise<TestUser> {
    const login = await request.post(`${baseURL}/api/public/login`, {
        data: { email: BOOTSTRAP_USER.email, password: BOOTSTRAP_USER.password }
    });
    const { token } = (await login.json()) as { token: string };

    const user: TestUser = {
        name: `E2E ${label}`,
        email: `e2e-${label}@example.com`,
        password: 'Passw0rd!23'
    };

    const created = await request.post(`${baseURL}/api/private/admin/user`, {
        headers: { Authorization: token },
        data: user
    });
    expect(
        created.status(),
        `admin user creation for ${user.email} returned ${created.status()}`
    ).toBe(200);

    return user;
}

import { APIRequestContext, expect } from '@playwright/test';
import { TestUser } from './user';

export async function tokenOf(
    request: APIRequestContext,
    baseURL: string,
    user: TestUser
): Promise<string> {
    const login = await request.post(`${baseURL}/api/public/login`, {
        data: { email: user.email, password: user.password }
    });
    return ((await login.json()) as { token: string }).token;
}

export async function createSprint(
    request: APIRequestContext,
    baseURL: string,
    token: string,
    idProject: number,
    body: Record<string, unknown>
): Promise<number> {
    const res = await request.post(`${baseURL}/api/private/project/${idProject}/sprint`, {
        headers: { Authorization: token },
        data: body
    });
    expect(res.status(), 'sprint creation').toBe(201);
    return ((await res.json()) as { idSprint: number }).idSprint;
}

export async function assignToSprint(
    request: APIRequestContext,
    baseURL: string,
    token: string,
    idProject: number,
    idIssuePublic: number,
    idSprint: number
): Promise<void> {
    const res = await request.patch(
        `${baseURL}/api/private/project/${idProject}/issue/${idIssuePublic}/sprint`,
        { headers: { Authorization: token }, data: { idSprint } }
    );
    expect(res.status(), 'sprint assignment').toBe(204);
}

export async function editIssue(
    request: APIRequestContext,
    baseURL: string,
    token: string,
    idProject: number,
    idIssuePublic: number,
    changes: Record<string, unknown>
): Promise<void> {
    const url = `${baseURL}/api/private/project/${idProject}/issue/${idIssuePublic}`;
    const current = await request.get(url, { headers: { Authorization: token } });
    expect(current.status(), 'reading the issue').toBe(200);
    const issue = (await current.json()) as Record<string, unknown>;

    const res = await request.patch(url, {
        headers: { Authorization: token },
        data: {
            idState: issue['idState'],
            idSeverity: issue['idSeverity'],
            title: issue['title'],
            description: issue['description'],
            estimated: issue['estimated'],
            points: issue['points'],
            ...changes
        }
    });
    expect(res.status(), 'editing the issue').toBe(200);
}

export async function stateIdOf(
    request: APIRequestContext,
    baseURL: string,
    token: string,
    idProject: number,
    name: string
): Promise<number> {
    const res = await request.get(`${baseURL}/api/private/state`, {
        headers: { Authorization: token }
    });
    expect(res.status(), 'reading project states').toBe(200);
    const states = (await res.json()) as { idState: number; idProject: number; name: string }[];
    const state = states.find(
        candidate => candidate.idProject === idProject && candidate.name === name
    );
    expect(state, `project has a "${name}" state`).toBeTruthy();
    return state!.idState;
}

export function isoDay(offsetDays: number): string {
    const now = new Date();
    const day = Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate());
    return new Date(day + offsetDays * 86_400_000).toISOString();
}

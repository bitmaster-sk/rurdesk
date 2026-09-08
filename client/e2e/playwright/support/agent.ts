import { APIRequestContext, expect, request as playwrightRequest } from '@playwright/test';

// Where each side reaches the stub gateway: the tracker calls it over the
// compose network, the test over the published port.
export const STUB_GATEWAY_URL_FOR_TRACKER = 'http://stub-gateway:9090';
export const STUB_GATEWAY_URL_FOR_TEST = 'http://localhost:9090';

export interface TestAgent {
    idUser: number;
    name: string;
}

/** Per-stage override of what the stub gateway reports; an omitted stage keeps the canned default. */
export interface StageScript {
    outcome?: 'output_submitted' | 'question_asked' | 'no_action_needed' | 'errored';
    message?: string;
    messageKind?: string;
    prUrl?: string;
    branchName?: string;
    errorReason?: string;
    errorDetail?: string;
    /** Report nothing for the stage, leaving the run in_progress. */
    stall?: boolean;
}

export type GatewayScript = Record<string, StageScript>;

/**
 * Creates an agent in the project and points it at the e2e stub gateway, then
 * hands the stub the agent's two one-time tokens. The agent does not exist when
 * the stack boots, so the stub cannot take them from its environment.
 */
export async function createStubGatewayAgent(
    request: APIRequestContext,
    baseURL: string,
    adminToken: string,
    idProject: number,
    label: string,
    script?: GatewayScript
): Promise<TestAgent> {
    const created = await request.post(`${baseURL}/api/private/admin/user`, {
        headers: { Authorization: adminToken },
        data: { name: `E2E Agent ${label}`, isAgent: true, idProject, role: 'member' }
    });
    expect(created.status(), 'creating the agent user').toBe(200);
    const agent = (await created.json()) as { idUser: number; name: string; rawKey: string };

    const gateway = await request.post(
        `${baseURL}/api/private/admin/user/${agent.idUser}/gateway`,
        {
            headers: { Authorization: adminToken },
            data: { gatewayUrl: STUB_GATEWAY_URL_FOR_TRACKER }
        }
    );
    expect(gateway.status(), 'creating the agent gateway').toBe(200);
    const { trackerToGatewayToken } = (await gateway.json()) as { trackerToGatewayToken: string };

    const stub = await playwrightRequest.newContext();
    try {
        const configured = await stub.post(`${STUB_GATEWAY_URL_FOR_TEST}/configure`, {
            data: {
                gatewayToTrackerToken: agent.rawKey,
                trackerToGatewayToken,
                script: script ?? {}
            }
        });
        expect(configured.status(), 'configuring the stub gateway').toBe(204);
    } finally {
        await stub.dispose();
    }

    return { idUser: agent.idUser, name: agent.name };
}

export async function assignAgent(
    request: APIRequestContext,
    baseURL: string,
    token: string,
    idProject: number,
    idIssuePublic: number,
    idUserAgent: number
): Promise<number> {
    const res = await request.post(
        `${baseURL}/api/private/project/${idProject}/issue/${idIssuePublic}/assign-agent`,
        { headers: { Authorization: token }, data: { idUserAgent } }
    );
    expect(res.status(), 'assigning the agent').toBe(200);
    return ((await res.json()) as { idRun: number }).idRun;
}

import { APIRequestContext, expect, request as playwrightRequest } from '@playwright/test';
import { STUB_GATEWAY_URL_FOR_TEST, STUB_GATEWAY_URL_FOR_TRACKER } from './agent-bot';

export interface EventMapping {
    event: string;
    idState: number;
}

export abstract class WorkflowEventMap {
    public static async createState(
        request: APIRequestContext,
        baseURL: string,
        token: string,
        idProject: number,
        name: string
    ): Promise<number> {
        const res = await request.post(`${baseURL}/api/private/state`, {
            headers: { Authorization: token },
            data: { idProject, name, start: false, final: false }
        });
        expect(res.status(), `creating state ${name}`).toBe(200);
        return ((await res.json()) as { idState: number }).idState;
    }

    /** Replaces the whole map, so events left out stay unmapped. */
    public static async mapEvents(
        request: APIRequestContext,
        baseURL: string,
        token: string,
        idProject: number,
        mappings: EventMapping[]
    ): Promise<void> {
        const res = await request.put(
            `${baseURL}/api/private/project/${idProject}/workflow-event-state-map`,
            { headers: { Authorization: token }, data: { mappings } }
        );
        expect(res.status(), `mapping ${mappings.map(m => m.event).join(', ')}`).toBe(200);
    }

    public static async mapEventToState(
        request: APIRequestContext,
        baseURL: string,
        token: string,
        idProject: number,
        event: string,
        idState: number
    ): Promise<void> {
        await WorkflowEventMap.mapEvents(request, baseURL, token, idProject, [{ event, idState }]);
    }

    public static repoPathFor(label: string): string {
        return `org/${label}`;
    }

    /** Without a git integration the implementation stage errors out instead of reaching pr_open. */
    public static async createGitIntegration(
        request: APIRequestContext,
        baseURL: string,
        token: string,
        idProject: number,
        label: string
    ): Promise<number> {
        const res = await request.post(
            `${baseURL}/api/private/project/${idProject}/git-integration`,
            {
                headers: { Authorization: token },
                data: {
                    name: `e2e-${label}`,
                    hostType: 'github',
                    baseUrl: STUB_GATEWAY_URL_FOR_TRACKER,
                    repoPath: WorkflowEventMap.repoPathFor(label),
                    accessToken: 'ghp_e2e_stub_token'
                }
            }
        );
        expect(res.status(), 'creating the git integration').toBe(201);
        return ((await res.json()) as { idGitIntegration: number }).idGitIntegration;
    }

    /** Flips what the fake git host reports for a repository. */
    public static async setPrState(
        repoPath: string,
        state: 'open' | 'closed',
        merged: boolean
    ): Promise<void> {
        const stub = await playwrightRequest.newContext();
        try {
            const res = await stub.post(`${STUB_GATEWAY_URL_FOR_TEST}/pr-state`, {
                data: { repoPath, state, merged }
            });
            expect(res.status(), `setting the pr state for ${repoPath}`).toBe(204);
        } finally {
            await stub.dispose();
        }
    }

    public static async issueState(
        request: APIRequestContext,
        baseURL: string,
        token: string,
        idProject: number,
        idIssuePublic: number
    ): Promise<number | null> {
        const res = await request.get(
            `${baseURL}/api/private/project/${idProject}/issue/${idIssuePublic}`,
            { headers: { Authorization: token } }
        );
        expect(res.status(), 'loading the issue').toBe(200);
        return ((await res.json()) as { idState: number | null }).idState;
    }

    public static async runPhase(
        request: APIRequestContext,
        baseURL: string,
        token: string,
        idRun: number
    ): Promise<string> {
        const res = await request.get(`${baseURL}/api/private/agent/run/${idRun}`, {
            headers: { Authorization: token }
        });
        expect(res.status(), 'loading the run').toBe(200);
        return ((await res.json()) as { phase: string }).phase;
    }

    public static async waitForRunPhase(
        request: APIRequestContext,
        baseURL: string,
        token: string,
        idRun: number,
        phase: string,
        timeoutMs = 60_000
    ): Promise<void> {
        await expect
            .poll(() => WorkflowEventMap.runPhase(request, baseURL, token, idRun), {
                timeout: timeoutMs,
                message: `run ${idRun} never reached phase ${phase}`
            })
            .toBe(phase);
    }

    public static async approveRun(
        request: APIRequestContext,
        baseURL: string,
        token: string,
        idRun: number
    ): Promise<void> {
        const res = await request.post(`${baseURL}/api/private/agent/run/${idRun}/approve`, {
            headers: { Authorization: token },
            data: {}
        });
        expect(res.status(), 'approving the run').toBe(200);
    }

    public static async restartRun(
        request: APIRequestContext,
        baseURL: string,
        token: string,
        idRun: number
    ): Promise<number> {
        const res = await request.post(`${baseURL}/api/private/agent/run/${idRun}/restart`, {
            headers: { Authorization: token },
            data: {}
        });
        expect(res.status(), 'restarting the run').toBe(200);
        return ((await res.json()) as { newIdRun: number }).newIdRun;
    }

    public static async cancelRun(
        request: APIRequestContext,
        baseURL: string,
        token: string,
        idRun: number
    ): Promise<void> {
        const res = await request.post(`${baseURL}/api/private/agent/run/${idRun}/cancel`, {
            headers: { Authorization: token },
            data: {}
        });
        expect(res.status(), 'cancelling the run').toBe(200);
    }
}
